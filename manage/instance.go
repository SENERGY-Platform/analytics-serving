/*
 * Copyright 2018 InfAI (CC SES)
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

package manage

import (
	"analytics-serving/db"
	"analytics-serving/lib"
	"analytics-serving/model"
	"analytics-serving/rancher-api"
	"log"

	"fmt"

	"strings"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/satori/go.uuid"
)

func CreateInstance(req lib.ServingRequest, userId string) {
	id := uuid.NewV4()
	instance := model.Instance{
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
	}
	dataFields := "{"
	var values []model.Value
	for index, value := range req.Values {
		values = append(values, model.Value{InstanceID: id, Name: value.Name, Type: value.Type, Path: value.Path})
		dataFields = dataFields + "\"" + value.Name + ":" + value.Type + "\":\"" + value.Path + "\""
		if index+1 < len(req.Values) {
			dataFields = dataFields + ","
		}
	}
	instance.Values = values
	dataFields = dataFields + "}"
	instance.RancherServiceId = rancher().CreateInstance(&instance, dataFields)
	db.DB.NewRecord(instance)
	db.DB.Create(&instance)
}

func GetInstance(id string) (instance model.Instance) {
	db.DB.Where("id = ?", id).Preload("Values").First(&instance)
	fmt.Println(instance)
	return
}

func GetInstances(id string, args map[string][]string) (instances model.Instances) {
	tx := db.DB.Where("user_id = ?", id)
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

func DeleteInstance(id string) bool {
	instance := model.Instance{}
	db.DB.Where("id = ?", id).First(&instance)
	rancher().DeleteInstance(instance.RancherServiceId)
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     lib.GetEnv("INFLUX_DB_HOST", ""),
		Username: lib.GetEnv("INFLUX_DB_USERNAME", ""),
		Password: lib.GetEnv("INFLUX_DB_PASSWORD", ""),
	})
	if err != nil {
		log.Fatal("Could not connect to InfluxDB")
	}
	_, errInflux := queryInfluxDB(c, fmt.Sprintf("DROP MEASUREMENT %s", `"`+instance.Measurement+`"`), instance.Database)
	if errInflux != nil {
		log.Fatal("Influx Error: ", errInflux)
	}
	db.DB.Delete(&instance)
	return true
}

func rancher() (RANCHER *rancher_api.Rancher) {
	rancher_api.Init()
	return rancher_api.RANCHER
}

func queryInfluxDB(clnt client.Client, cmd string, db string) (res []client.Result, err error) {
	q := client.Query{
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
