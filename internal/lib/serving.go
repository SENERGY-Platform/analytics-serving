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
	"fmt"
	"github.com/google/uuid"
	_ "github.com/influxdata/influxdb1-client"
	"github.com/kr/pretty"

	"strings"
	"time"
)

type Serving struct {
	driver Driver
	influx *Influx
}

func NewServing(driver Driver) *Serving {
	influx := NewInflux()
	return &Serving{driver, influx}
}

func (f *Serving) CreateInstance(req ServingRequest, userId string) (instance Instance) {
	id := uuid.New()
	appId := uuid.New()
	instance = f.createInstanceWithId(id, appId, req, userId)
	return
}

func (f *Serving) createInstanceWithId(id uuid.UUID, appId uuid.UUID, req ServingRequest, userId string) Instance {
	instance, dataFields, tagFields := populateInstance(id, appId, req, userId)
	for {
		serviceId, err := f.driver.CreateInstance(&instance, dataFields, tagFields)
		if err == nil {
			instance.RancherServiceId = serviceId
			break
		}
		fmt.Println(err)
		fmt.Println("Retrying in ...")
		fmt.Println("3")
		time.Sleep(1 * time.Second)
		fmt.Println("2")
		time.Sleep(1 * time.Second)
		fmt.Println("1")
		time.Sleep(1 * time.Second)
	}
	DB.NewRecord(instance)
	DB.Create(&instance)
	return instance
}

func (f *Serving) UpdateInstance(id string, userId string, request ServingRequest) (instance Instance, errors []error) {
	errors = DB.Where("id = ? AND user_id = ?", id, userId).Preload("Values").First(&instance).GetErrors()
	if len(errors) > 0 {
		return
	}
	uid, _ := uuid.Parse(id)
	appId := instance.ApplicationId
	if appId.ID() == 0 {
		appId = uuid.New()
	}
	requestInstance, _, _ := populateInstance(uid, appId, request, userId)
	requestInstance.RancherServiceId = instance.RancherServiceId
	requestInstance.CreatedAt = instance.CreatedAt
	requestInstance.UpdatedAt = instance.UpdatedAt
	if request.ForceUpdate {
		for {
			_, errors = f.DeleteInstanceForUser(id, userId)
			if len(errors) < 1 {
				instance = f.createInstanceWithId(uid, appId, request, userId)
				break
			}
		}
	} else {
		change := pretty.Diff(instance, requestInstance)
		if len(change) > 0 {
			if len(change) > 2 {
				for {
					_, errors = f.DeleteInstanceForUser(id, userId)
					if len(errors) < 1 {
						instance = f.createInstanceWithId(uid, appId, request, userId)
						break
					}
				}
			} else {
				if len(change) == 1 {
					if instance.Name != requestInstance.Name || instance.Description != requestInstance.Description {
						errors = DB.Model(&instance).UpdateColumns(Instance{
							Name:        requestInstance.Name,
							Description: requestInstance.Description}).GetErrors()
					} else {
						for {
							_, errors = f.DeleteInstanceForUser(id, userId)
							if len(errors) < 1 {
								instance = f.createInstanceWithId(uid, appId, request, userId)
								break
							}
						}
					}
				} else {
					if instance.Name != requestInstance.Name && instance.Description != requestInstance.Description {
						errors = DB.Model(&instance).UpdateColumns(Instance{
							Name:        requestInstance.Name,
							Description: requestInstance.Description}).GetErrors()
					} else {
						for {
							_, errors = f.DeleteInstanceForUser(id, userId)
							if len(errors) < 1 {
								instance = f.createInstanceWithId(uid, appId, request, userId)
								break
							}
						}
					}
				}
			}
		}
	}
	return
}

func (f *Serving) GetInstance(id string, userId string) (instance Instance, errors []error) {
	errors = DB.Where("id = ? AND user_id = ?", id, userId).Preload("Values").First(&instance).GetErrors()
	return
}

func (f *Serving) GetInstancesForUser(userId string, args map[string][]string) (instances Instances) {
	return f.GetInstances(userId, args, false)
}

func (f *Serving) GetInstances(userId string, args map[string][]string, admin bool) (instances Instances) {
	tx := DB.Select("*").Where("user_id = ?", userId)
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
				}
			} else {
				tx = tx.Where("name LIKE ?", "%"+value[0]+"%")
			}

		}
		if arg == "generated" {
			if value[0] == "true" {
				tx = tx.Where("`generated` = TRUE")
			} else {
				tx = tx.Where("`generated` = FALSE")
			}
		}
	}
	tx.Preload("Values").Find(&instances)
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
	errors = tx.First(&instance).GetErrors()
	if len(errors) > 0 {
		fmt.Println(errors)
		return
	}
	if GetEnv("DRIVER", "rancher") == "rancher2" {
		err := f.driver.DeleteInstance(instance.ID.String())
		errors = append(errors, err)
	} else {
		err := f.driver.DeleteInstance(instance.RancherServiceId)
		errors = append(errors, err)
	}

	errors = f.influx.forceDeleteMeasurement(id, userId, instance)

	DB.Delete(&instance)
	return true, errors
}

func populateInstance(id uuid.UUID, appId uuid.UUID, req ServingRequest, userId string) (instance Instance, dataFields string, tagFields string) {
	instance = Instance{
		ID:            id,
		Measurement:   id.String(),
		Name:          req.Name,
		ApplicationId: appId,
		Description:   req.Description,
		EntityName:    req.EntityName,
		ServiceName:   req.ServiceName,
		Topic:         req.Topic,
		Filter:        req.Filter,
		FilterType:    req.FilterType,
		UserId:        userId,
		Database:      userId,
		TimePath:      req.TimePath,
		Offset:        req.Offset,
		Generated:     req.Generated,
	}
	if req.TimePrecision != "" {
		instance.TimePrecision = &req.TimePrecision
	}

	instance.Values, dataFields = transformServingValues(id, req.Values)
	instance.Tags, tagFields = transformServingValues(id, req.Tags)

	return
}

func transformServingValues(id uuid.UUID, requestValues []ServingRequestValue) (values []Value, fields string) {
	fields = "{"
	for index, value := range requestValues {
		values = append(values, Value{InstanceID: id, Name: value.Name, Type: value.Type, Path: value.Path})
		fields = fields + "\"" + value.Name + ":" + value.Type + "\":\"" + value.Path + "\""
		if index+1 < len(requestValues) {
			fields = fields + ","
		}
	}
	fields = fields + "}"
	return
}
