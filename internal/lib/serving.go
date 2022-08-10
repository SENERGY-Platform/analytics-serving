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
	"github.com/google/uuid"
	_ "github.com/influxdata/influxdb1-client"
	"github.com/jinzhu/gorm"
	"log"
	"strings"
	"time"
)

type Serving struct {
	driver                 Driver
	influx                 *Influx
	permissionService      PermissionApiService
	pipelineService        PipelineApiService
	importDeployService    ImportDeployService
	exportDatabaseIdPrefix string
}

func NewServing(driver Driver, permissionService PermissionApiService, pipelineService PipelineApiService, importDeployService ImportDeployService, exportDatabaseIdPrefix string) *Serving {
	influx := NewInflux()
	return &Serving{driver, influx, permissionService, pipelineService, importDeployService, exportDatabaseIdPrefix}
}

func (f *Serving) CreateInstance(req ServingRequest, userId string, token string) (instance Instance, err error) {
	access, err := f.userHasSourceAccess(req, token)
	if !access {
		return
	}
	id := uuid.New()
	appId := uuid.New()
	instance, err = f.createInstanceWithId(id, appId, req, userId)
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
	errors = DB.Where("id = ? AND user_id = ?", id, userId).Preload("Values").Preload("ExportDatabase").First(&instance).GetErrors()
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
		_, errs := f.DeleteInstanceForUser(id, userId)
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

func (f *Serving) GetInstance(id string, userId string) (instance Instance, errors []error) {
	errors = DB.Where("id = ? AND user_id = ?", id, userId).Preload("Values").Preload("ExportDatabase").First(&instance).GetErrors()
	return
}

func (f *Serving) GetInstancesForUser(userId string, args map[string][]string) (instances Instances, count int64, errors []error) {
	return f.GetInstances(userId, args, false)
}

func (f *Serving) GetInstances(userId string, args map[string][]string, admin bool) (instances Instances, total int64, errors []error) {
	tx := DB.Select("*").Where("user_id = ?", userId)
	countTx := DB.Where("user_id = ?", userId)
	if admin {
		tx = DB.Select("*")
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

func (f *Serving) DeleteInstancesForUser(ids []string, userId string) (deleted []string, errors []error) {
	for _, id := range ids {
		success, errs := f.DeleteInstance(id, userId, false)
		if success {
			deleted = append(deleted, id)
		} else {
			errors = append(errors, errs...)
		}
	}
	return
}

func (f *Serving) DeleteInstanceForUser(id string, userId string) (deleted bool, errors []error) {
	return f.DeleteInstance(id, userId, false)
}

func (f *Serving) DeleteInstance(id string, userId string, admin bool) (deleted bool, errors []error) {
	deleted = false
	instance := Instance{}
	tx := DB.Where("id = ? AND user_id = ?", id, userId)
	if admin {
		tx = DB.Where("id = ?", id)
	}
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
			errs := f.influx.forceDeleteMeasurement(id, userId, instance)
			if len(errs) > 0 {
				for _, e := range errs {
					errors = append(errors, e)
				}
			}
		}
	}
	return
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

func (f *Serving) GetExportDatabases(userId string, args map[string][]string) (databases []ExportDatabase, errs []error) {
	tx := DB.Select("*").Where("public = TRUE OR user_id = ?", userId)
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
				allowed := []string{"name", "description", "type"}
				if StringInSlice(search[0], allowed) {
					tx = tx.Where(search[0]+" LIKE ?", "%"+search[1]+"%")
				}
			} else {
				tx = tx.Where("name LIKE ?", "%"+value[0]+"%")
			}
		}
		if arg == "deployment" {
			tx = tx.Where("`deployment` = ?", value[0])
		}
		if arg == "public" {
			if value[0] == "true" {
				tx = tx.Where("`public` = TRUE")
			} else {
				tx = tx.Where("`public` = FALSE")
			}
		}
		if arg == "owner" {
			if value[0] == "true" {
				tx = tx.Where("`user_id` = ?", userId)
			} else {
				tx = tx.Where("`user_id` != ?", userId)
			}
		}
	}
	errs = tx.Find(&databases).GetErrors()
	if len(errs) > 0 {
		for _, err := range errs {
			if gorm.IsRecordNotFoundError(err) {
				return databases, nil
			}
		}
		log.Println("listing export-databases failed - " + fmt.Sprint(errs))
		return
	}
	return
}

func (f *Serving) GetExportDatabase(id string, userId string) (database ExportDatabase, errs []error) {
	errs = DB.Where("id = ? AND (user_id = ? OR public = TRUE)", id, userId).First(&database).GetErrors()
	if len(errs) > 0 {
		log.Println("retrieving export-database failed - " + id + " - " + fmt.Sprint(errs))
		return
	}
	return
}

func (f *Serving) CreateExportDatabase(id string, req ExportDatabaseRequest, userId string) (database ExportDatabase, errs []error) {
	if id == "" {
		id = uuid.New().String()
		if f.exportDatabaseIdPrefix != "" {
			id = f.exportDatabaseIdPrefix + id
		}
	}
	database = populateExportDatabase(id, req, userId)
	if driver, ok := f.driver.(ExportWorkerKafkaApi); ok {
		err := driver.CreateFilterTopic(database.EwFilterTopic, true)
		if err != nil {
			errs = append(errs, err)
			return
		}
	}
	DB.NewRecord(database)
	errs = DB.Create(&database).GetErrors()
	if len(errs) > 0 {
		log.Println("creating export-database failed - " + fmt.Sprint(errs))
		return
	}
	log.Println("successfully created export-database - " + database.ID)
	return
}

func (f *Serving) UpdateExportDatabase(id string, req ExportDatabaseRequest, userId string) (database ExportDatabase, errs []error) {
	errs = DB.Where("id = ? AND user_id = ?", id, userId).First(&database).GetErrors()
	if len(errs) > 0 {
		for _, err := range errs {
			if gorm.IsRecordNotFoundError(err) {
				database, errs = f.CreateExportDatabase(id, req, userId)
				return
			}
		}
		log.Println("updating export-database failed - " + id + " - " + fmt.Sprint(errs))
		return
	}
	database = populateExportDatabase(id, req, userId)
	errs = DB.Save(&database).GetErrors()
	if len(errs) > 0 {
		log.Println("updating export-database failed - " + id + " - " + fmt.Sprint(errs))
		return
	}
	log.Println("successfully updated export-database - " + database.ID)
	return
}

func (f *Serving) DeleteExportDatabase(id string, userId string) (errs []error) {
	var database ExportDatabase
	errs = DB.Where("id = ? AND user_id = ?", id, userId).First(&database).GetErrors()
	if len(errs) > 0 {
		log.Println("deleting export-database failed - " + id + " - " + fmt.Sprint(errs))
		return
	}
	errs = DB.Delete(&database).GetErrors()
	if len(errs) > 0 {
		log.Println("deleting export-database failed - " + id + " - " + fmt.Sprint(errs))
	}
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

func populateExportDatabase(id string, req ExportDatabaseRequest, userId string) (database ExportDatabase) {
	database = ExportDatabase{
		ID:            id,
		Name:          req.Name,
		Description:   req.Description,
		Type:          req.Type,
		Deployment:    req.Deployment,
		Url:           req.Url,
		EwFilterTopic: req.EwFilterTopic,
		Public:        req.Public,
		UserId:        userId,
	}
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
