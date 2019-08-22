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

	influxClient "github.com/influxdata/influxdb/client/v2"

	uuid "github.com/satori/go.uuid"

	"strings"
)

type Serving struct {
	driver Driver
}

func NewServing(driver Driver) *Serving {
	return &Serving{driver}
}

func (f *Serving) CreateInstance(req ServingRequest, userId string) {
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
}

func (f *Serving) GetInstance(id string) (instance Instance) {
	DB.Where("id = ?", id).Preload("Values").First(&instance)
	return
}

func (f *Serving) GetInstances(id string, args map[string][]string) (instances Instances) {
	tx := DB.Where("user_id = ?", id)
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

func (f *Serving) DeleteInstance(id string) bool {
	instance := Instance{}
	DB.Where("id = ?", id).First(&instance)
	if GetEnv("DRIVER", "rancher") == "rancher2" {
		e := f.driver.DeleteInstance(instance.ID.String())
		if e != nil {
			fmt.Println(e)
		}
	} else {
		e := f.driver.DeleteInstance(instance.RancherServiceId)
		if e != nil {
			fmt.Println(e)
		}
	}
	c, err := influxClient.NewHTTPClient(influxClient.HTTPConfig{
		Addr:     "http://" + GetEnv("INFLUX_DB_HOST", "") + ":" + GetEnv("INFLUX_DB_PORT", "8086"),
		Username: GetEnv("INFLUX_DB_USERNAME", "root"),
		Password: GetEnv("INFLUX_DB_PASSWORD", ""),
	})
	if err != nil {
		fmt.Println("could not connect to InfluxDB")
	}
	_, err = queryInfluxDB(c, fmt.Sprintf("DROP MEASUREMENT %s", `"`+instance.Measurement+`"`), instance.Database)
	if err != nil {
		fmt.Println("influx Error: ", err)
	}
	DB.Delete(&instance)
	return true
}

func queryInfluxDB(clnt influxClient.Client, cmd string, db string) (res []influxClient.Result, err error) {
	q := influxClient.Query{
		Command:  cmd,
		Database: db,
	}
	if response, err := clnt.Query(q); err == nil {
		if response.Error() != nil {
			return res, response.Error()
		}
		res = response.Results
	} else {
		return res, err
	}
	return res, nil
}
