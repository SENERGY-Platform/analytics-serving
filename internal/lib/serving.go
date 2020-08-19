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
	_ "github.com/influxdata/influxdb1-client"
	uuid "github.com/satori/go.uuid"
	"strings"
)

type Serving struct {
	driver Driver
	influx *Influx
}

func NewServing(driver Driver) *Serving {
	influx := NewInflux()
	return &Serving{driver, influx}
}

func (f *Serving) CreateInstance(req ServingRequest, userId string) Instance {
	id := uuid.NewV4()
	instance := Instance{
		ID:          id,
		Measurement: id.String(),
		Name:        req.Name,
		Description: req.Description,
		EntityName:  req.EntityName,
		ServiceName: req.ServiceName,
		Topic:       req.Topic,
		Filter:      req.Filter,
		FilterType:  req.FilterType,
		UserId:      userId,
		Database:    userId,
		TimePath:    req.TimePath,
		Offset:      req.Offset,
	}
	if req.TimePrecision != "" {
		instance.TimePrecision = &req.TimePrecision
	}
	dataFields := "{"
	var values []Value
	for index, value := range req.Values {
		values = append(values, Value{InstanceID: id, Name: value.Name, Type: value.Type, Path: value.Path})
		dataFields = dataFields + "\"" + value.Name + ":" + value.Type + "\":\"" + value.Path + "\""
		if index+1 < len(req.Values) {
			dataFields = dataFields + ","
		}
	}
	instance.Values = values
	dataFields = dataFields + "}"

	instance.RancherServiceId = f.driver.CreateInstance(&instance, dataFields)

	DB.NewRecord(instance)
	DB.Create(&instance)
	return instance
}

func (f *Serving) UpdateInstance(id string, userId string, request ServingRequest) (instance Instance, errors []error) {
	errors = DB.Where("id = ? AND user_id = ?", id, userId).First(&instance).GetErrors()
	if len(errors) > 0 {
		return
	}
	if instance.Name != request.Name || instance.Description != request.Description {
		errors = DB.Model(&instance).Updates(map[string]interface{}{"name": request.Name, "description": request.Description}).GetErrors()
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
			order := strings.Split(value[0], ":")
			tx = tx.Order(order[0] + " " + order[1])
		}
		if arg == "search" {
			tx = tx.Where("name LIKE ?", "%"+value[0]+"%")
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

	for {
		errors = f.influx.DropMeasurement(instance)
		if len(errors) > 0 {
			return
		}
		measurements, err := f.influx.GetMeasurements(userId)
		if err != nil {
			errors = append(errors, err)
			return
		}
		if !StringInSlice(id, measurements) {
			break
		}
	}

	DB.Delete(&instance)
	return true, errors
}
