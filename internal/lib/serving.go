/*
 * Copyright 2019 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package lib

import (
	"errors"
	"fmt"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/google/uuid"
	_ "github.com/influxdata/influxdb1-client"
	"github.com/jinzhu/gorm"
	"github.com/robfig/cron/v3"
	"log"
	"strings"
	"sync"
	"time"
)

type Serving struct {
	driver                 Driver
	influx                 Influx
	permissionService      PermissionApiService
	pipelineService        PipelineApiService
	importDeployService    ImportDeployService
	exportDatabaseIdPrefix string
	permissionsV2          permV2Client.Client
	permMux                sync.RWMutex
}

func NewServing(driver Driver, influx Influx, permissionService PermissionApiService, pipelineService PipelineApiService, importDeployService ImportDeployService, exportDatabaseIdPrefix string, permissionsV2 permV2Client.Client, cleanupChron string, cleanupRecheckWait time.Duration) (*Serving, error) {
	if influx == nil {
		influx = NewInflux()
	}
	if permissionsV2 != nil {
		_, err, _ := permissionsV2.SetTopic(permV2Client.InternalAdminToken, permV2Client.Topic{
			Id:     ExportInstancePermissionsTopic,
			NoCqrs: true,
		})
		if err != nil {
			return nil, err
		}
	}
	result := &Serving{
		driver:                 driver,
		influx:                 influx,
		permissionService:      permissionService,
		pipelineService:        pipelineService,
		importDeployService:    importDeployService,
		exportDatabaseIdPrefix: exportDatabaseIdPrefix,
		permissionsV2:          permissionsV2,
	}
	if cleanupChron != "" && cleanupChron != "-" {
		_, err := cron.New().AddFunc(cleanupChron, func() {
			err := result.ExportInstanceCleanup(cleanupRecheckWait)
			if err != nil {
				log.Println("WARNING: cleanup fail", err)
			}
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

const ExportInstancePermissionsTopic = "export-instances"

func (f *Serving) CreateInstance(req ServingRequest, userId string, token string) (instance Instance, err error) {
	access, err := f.userHasSourceAccess(req, token)
	if !access {
		return
	}
	id := uuid.New()
	appId := uuid.New()

	if f.permissionsV2 != nil {
		f.permMux.RLock()
		defer f.permMux.RUnlock()
		_, err, _ = f.permissionsV2.SetPermission(
			token,
			ExportInstancePermissionsTopic,
			id.String(),
			permV2Client.ResourcePermissions{
				UserPermissions:  map[string]permV2Client.PermissionsMap{userId: {Read: true, Write: true, Execute: true, Administrate: true}},
				GroupPermissions: map[string]permV2Client.PermissionsMap{},
			},
			permV2Client.SetPermissionOptions{Wait: true})
		if err != nil {
			return instance, err
		}
	}

	instance, err = f.createInstanceWithId(id, appId, req, userId)
	if err != nil {
		if f.permissionsV2 != nil {
			temperr, _ := f.permissionsV2.RemoveResource(token, ExportInstancePermissionsTopic, id.String())
			log.Printf("ERROR: %v --> try to remove now inconsistent permission: %v\n", err, temperr)
		}
		return instance, err
	}
	return
}

func (f *Serving) createInstanceWithId(id uuid.UUID, appId uuid.UUID, req ServingRequest, userId string) (instance Instance, err error) {
	database, errs := f.GetExportDatabase(req.ExportDatabaseID, userId)
	if len(errs) > 0 {
		err = errors.New("export-database does not exist or user unauthorized")
		return
	}
	instance, dataFields, tagFields := populateInstance(id, appId, req, userId)
	instance.ExportDatabase = database
	err = retry(5, 5*time.Second, func() (err error) {
		serviceId, err := f.driver.CreateInstance(&instance, dataFields, tagFields)
		if err == nil {
			instance.RancherServiceId = serviceId
		}
		return
	})
	if err != nil {
		return
	} else {
		DB.NewRecord(instance)
		errs = DB.Create(&instance).GetErrors()
		if len(errs) > 0 {
			err2 := retry(5, 5*time.Second, func() (e error) {
				e = f.driver.DeleteInstance(&instance)
				return
			})
			if err2 != nil {
				errs = append(errs, err2)
			}
			err = errors.New("serving - creating export failed - " + fmt.Sprint(errs))
			log.Println(err)
			return
		}
		log.Println("serving - successfully created export - " + instance.ID.String())
	}
	return
}

func (f *Serving) UpdateInstance(id string, userId string, request ServingRequest, token string) (instance Instance, errors []error) {
	access, err := f.userHasSourceAccess(request, token)
	if !access {
		errors = append(errors, err)
		return
	}
	query := DB.Where("id = ? AND user_id = ?", id, userId)
	if f.permissionsV2 != nil {
		access, err, _ = f.permissionsV2.CheckPermission(token, ExportInstancePermissionsTopic, id, permV2Client.Write)
		if err != nil {
			return instance, []error{err}
		}
		if !access {
			return instance, []error{fmt.Errorf("access denied")}
		}
		query = DB.Where("id = ?", id)
	}
	errors = query.Preload("Values").Preload("ExportDatabase").First(&instance).GetErrors()
	if len(errors) > 0 {
		return
	}
	uid, _ := uuid.Parse(id)
	appId := instance.ApplicationId
	if appId.ID() == 0 {
		appId = uuid.New()
	}
	if request.Offset != instance.Offset {
		appId = uuid.New()
	}
	requestInstance, _, _ := populateInstance(uid, appId, request, userId)
	requestInstance.RancherServiceId = instance.RancherServiceId
	requestInstance.CreatedAt = instance.CreatedAt
	instance = f.update(id, userId, request, requestInstance, uid, appId)
	return
}

func (f *Serving) update(id string, userId string, request ServingRequest, instance Instance, uid uuid.UUID, appId uuid.UUID) Instance {
	err := retry(5, 5*time.Second, func() (err error) {
		//we use an empty userId to indicate that the user should not be checked
		//the check has already been done by UpdateInstance()
		_, errs := f.deleteInstance(id, "")
		if len(errs) > 1 {
			err = errs[0]
		}
		return err
	})
	if err != nil {
		log.Println(err)
	} else {
		instance, _ = f.createInstanceWithId(uid, appId, request, userId)
		log.Println("serving - successfully updated export - " + instance.ID.String())
	}
	return instance
}

func (f *Serving) GetInstance(id string, userId string, token string) (instance Instance, errors []error) {
	query := DB.Where("id = ? AND user_id = ?", id, userId)
	if f.permissionsV2 != nil {
		access, err, _ := f.permissionsV2.CheckPermission(token, ExportInstancePermissionsTopic, id, permV2Client.Read)
		if err != nil {
			return instance, []error{err}
		}
		if !access {
			return instance, []error{fmt.Errorf("access denied")}
		}
		query = DB.Where("id = ?", id)
	}
	errors = query.Preload("Values").Preload("ExportDatabase").First(&instance).GetErrors()
	return
}

func (f *Serving) getInstanceById(id string) (instance Instance, err error) {
	query := DB.Where("id = ?", id)
	err = errors.Join(query.Preload("Values").Preload("ExportDatabase").First(&instance).GetErrors()...)
	return
}

func (f *Serving) GetInstancesForUser(userId string, args map[string][]string, token string) (instances Instances, count int64, errors []error) {
	return f.GetInstances(userId, args, false, token)
}

func (f *Serving) GetInstances(userId string, args map[string][]string, admin bool, token string) (instances Instances, total int64, errors []error) {
	tx := DB.Select("*")
	countTx := DB
	if !admin {
		tx = DB.Select("*").Where("user_id = ?", userId)
		countTx = DB.Where("user_id = ?", userId)
		if f.permissionsV2 != nil {
			ids, err, _ := f.permissionsV2.ListAccessibleResourceIds(token, ExportInstancePermissionsTopic, permV2Client.ListOptions{}, permV2Client.Read)
			if err != nil {
				return instances, total, []error{err}
			}
			if len(ids) == 0 {
				return Instances{}, 0, nil
			}
			tx = DB.Select("*").Where("id IN (?)", ids)
			countTx = DB.Where("id IN (?)", ids)
		}
	}
	for arg, value := range args {
		if arg == "limit" {
			tx = tx.Limit(value[0])
		}
		if arg == "offset" {
			tx = tx.Offset(value[0])
		}
		if arg == "order" {
			order := strings.SplitN(value[0], ":", 2)
			tx = tx.Order(order[0] + " " + order[1])
		}
		if arg == "search" {
			search := strings.SplitN(value[0], ":", 2)
			if len(search) > 1 {
				allowed := []string{"name", "description", "entity_name", "service_name"}
				if StringInSlice(search[0], allowed) {
					tx = tx.Where(search[0]+" LIKE ?", "%"+search[1]+"%")
					countTx = countTx.Where(search[0]+" LIKE ?", "%"+search[1]+"%")
				}
			} else {
				tx = tx.Where("name LIKE ?", "%"+value[0]+"%")
				countTx = countTx.Where("name LIKE ?", "%"+value[0]+"%")
			}

		}
		if arg == "generated" {
			if value[0] == "true" {
				tx = tx.Where("`generated` = TRUE")
				countTx = countTx.Where("`generated` = TRUE")
			} else {
				tx = tx.Where("`generated` = FALSE")
				countTx = countTx.Where("`generated` = FALSE")
			}
		}
		if arg == "export_database_id" {
			tx = tx.Where("export_database_id = ?", value[0])
			countTx = countTx.Where("export_database_id = ?", value[0])
		}
		if arg == "internal_only" {
			if value[0] == "true" {
				tx = tx.Where("export_database_id IN (?)", DB.Table("export_databases").Select("id").Where("deployment = ? AND (public = TRUE OR user_id = ?)", "internal", userId).SubQuery())
				countTx = countTx.Where("export_database_id IN (?)", DB.Table("export_databases").Select("id").Where("deployment = ? AND (public = TRUE OR user_id = ?)", "internal", userId).SubQuery())
			}
		}
	}
	errors = tx.Preload("Values").Preload("ExportDatabase").Find(&instances).GetErrors()
	countTx.Find(&Instances{}).Count(&total)
	return
}

func (f *Serving) DeleteInstancesForUser(ids []string, userId string, token string) (deleted []string, errors []error) {
	for _, id := range ids {
		success, errs := f.DeleteInstanceWithPermHandling(id, userId, false, token)
		if success {
			deleted = append(deleted, id)
		} else {
			errors = append(errors, errs...)
		}
	}
	return
}

func (f *Serving) DeleteInstanceForUser(id string, userId string, token string) (deleted bool, errors []error) {
	return f.DeleteInstanceWithPermHandling(id, userId, false, token)
}

func (f *Serving) DeleteInstanceWithPermHandling(id string, userId string, admin bool, token string) (deleted bool, errors []error) {
	if f.permissionsV2 != nil {
		f.permMux.RLock()
		defer f.permMux.RUnlock()
	}
	if admin {
		userId = ""
	}
	if f.permissionsV2 != nil && !admin {
		userId = ""
		access, err, _ := f.permissionsV2.CheckPermission(token, ExportInstancePermissionsTopic, id, permV2Client.Administrate)
		if err != nil {
			return deleted, []error{err}
		}
		if !access {
			return deleted, []error{fmt.Errorf("access denied")}
		}
	}
	deleted, errors = f.deleteInstance(id, userId)
	if len(errors) > 0 {
		return deleted, errors
	}
	if f.permissionsV2 != nil && deleted {
		err := retry(5, 5*time.Second, func() (err error) {
			err, _ = f.permissionsV2.RemoveResource(token, ExportInstancePermissionsTopic, id)
			return
		})
		if err != nil {
			return deleted, []error{err}
		}
	}
	return
}

func (f *Serving) deleteInstance(id string, userId string) (deleted bool, errors []error) {
	tx := DB.Where("id = ?", id)
	if userId != "" {
		tx = DB.Where("id = ? AND user_id = ?", id, userId)
	}
	deleted = false
	instance := Instance{}

	errors = tx.Preload("ExportDatabase").First(&instance).GetErrors()
	if len(errors) > 0 {
		for _, e := range errors {
			if gorm.IsRecordNotFoundError(e) {
				errors = nil
				return
			}
		}
		return
	}
	err := retry(5, 5*time.Second, func() (err error) {
		err = f.driver.DeleteInstance(&instance)
		return
	})
	if err != nil {
		errors = append(errors, err)
		return
	} else {
		deleted = true
		errors = DB.Delete(&instance).GetErrors()
		if instance.ExportDatabase.Type == "influxdb" {
			errs := f.influx.ForceDeleteMeasurement(id, userId, instance)
			if len(errs) > 0 {
				for _, e := range errs {
					errors = append(errors, e)
				}
			}
		}
	}
	return deleted, errors
}

func (f *Serving) CreateFromInstance(instance *Instance) (err error) {
	var servingRequestValues []ServingRequestValue
	for _, value := range instance.Values {
		servingRequestValues = append(servingRequestValues, ServingRequestValue{
			Name: value.Name,
			Type: value.Type,
			Path: value.Path,
			Tag:  value.Tag,
		})
	}
	var dataFields, tagFields string
	_, dataFields, tagFields = transformServingValues(instance.ID, servingRequestValues)
	instance.RancherServiceId, err = f.driver.CreateInstance(instance, dataFields, tagFields)
	return
}

func (f *Serving) userHasSourceAccess(req ServingRequest, token string) (access bool, err error) {
	access = false
	switch req.FilterType {
	case "deviceId":
		hasAccess, e := f.permissionService.UserHasDevicesReadAccess([]string{req.Filter}, token)
		if e != nil {
			return access, e
		}
		if !hasAccess {
			e = errors.New("serving - user does not have the rights to access the devices")
			return access, e
		}
		break
	case "operatorId":
		hasAccess, e := f.pipelineService.UserHasPipelineAccess(strings.Split(req.Filter, ":")[0], token)
		if e != nil {
			e = errors.New("serving - user does not have the rights to access the pipeline: " + strings.Split(req.Filter, ":")[0])
			return access, e
		}
		if !hasAccess {
			e = errors.New("serving - user does not have the rights to access the pipeline")
			return access, e
		}
		break
	case "import_id":
		hasAccess, e := f.importDeployService.UserHasImportAccess(req.Filter, token)
		if e != nil {
			e = errors.New("serving - user does not have the rights to access the import: " + req.Filter)
			return access, e
		}
		if !hasAccess {
			e = errors.New("serving - user does not have the rights to access the import")
			return access, e
		}
		break
	default:
		return false, err
	}
	access = true
	return
}

func populateInstance(id uuid.UUID, appId uuid.UUID, req ServingRequest, userId string) (instance Instance, dataFields string, tagFields string) {
	instance = Instance{
		ID:               id,
		Measurement:      id.String(),
		Name:             req.Name,
		ApplicationId:    appId,
		Description:      req.Description,
		EntityName:       req.EntityName,
		ServiceName:      req.ServiceName,
		Topic:            req.Topic,
		Filter:           req.Filter,
		FilterType:       req.FilterType,
		UserId:           userId,
		Database:         userId,
		TimePath:         req.TimePath,
		Offset:           req.Offset,
		Generated:        req.Generated,
		ExportDatabaseID: req.ExportDatabaseID,
		TimestampFormat:  req.TimestampFormat,
	}
	if req.TimePrecision != "" {
		instance.TimePrecision = &req.TimePrecision
	}

	instance.Values, dataFields, tagFields = transformServingValues(id, req.Values)

	return
}

func transformServingValues(id uuid.UUID, requestValues []ServingRequestValue) (values []Value, dataFields string, tagFields string) {
	dataFields = "{"
	tagFields = "{"
	for _, value := range requestValues {
		values = append(values, Value{InstanceID: id, Name: value.Name, Type: value.Type, Path: value.Path, Tag: value.Tag})
		if value.Tag {
			if len(tagFields) > 1 {
				tagFields = tagFields + ","
			}
			tagFields = tagFields + "\"" + value.Name + ":" + value.Type + "\":\"" + value.Path + "\""
		} else {
			if len(dataFields) > 1 {
				dataFields = dataFields + ","
			}
			dataFields = dataFields + "\"" + value.Name + ":" + value.Type + "\":\"" + value.Path + "\""
		}
	}
	dataFields = dataFields + "}"
	tagFields = tagFields + "}"
	return
}
