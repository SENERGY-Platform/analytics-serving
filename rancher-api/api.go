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

package rancher_api

import (
	"analytics-serving/lib"
	"fmt"
	"net/http"

	"analytics-serving/model"

	"github.com/parnurzeal/gorequest"
)

var RANCHER *Rancher

func Init() {
	r := NewRancher(
		lib.GetEnv("RANCHER_ENDPOINT", ""),
		lib.GetEnv("RANCHER_ACCESS_KEY", ""),
		lib.GetEnv("RANCHER_SECRET_KEY", ""),
		lib.GetEnv("RANCHER_STACK_ID", ""),
	)
	RANCHER = r
}

type Rancher struct {
	url       string
	accessKey string
	secretKey string
	stackId   string
}

func NewRancher(url string, accessKey string, secretKey string, stackId string) *Rancher {
	return &Rancher{url, accessKey, secretKey, stackId}
}

func (r Rancher) CreateInstance(instance *model.Instance, dataFields string) string {
	env := map[string]string{
		"KAFKA_GROUP_ID":      "transfer-" + instance.ID.String(),
		"KAFKA_BOOTSTRAP":     lib.GetEnv("KAFKA_BOOTSTRAP", "broker.kafka.rancher.internal:9092"),
		"KAFKA_TOPIC":         instance.Topic,
		"DATA_MEASUREMENT":    instance.Measurement,
		"DATA_FIELDS_MAPPING": dataFields,
		"DATA_TIME_MAPPING":   instance.TimePath,
		"DATA_FILTER_ID":      instance.Filter,
		"INFLUX_DB":           instance.Database,
		"INFLUX_HOST":         "influxdb",
		"INFLUX_USER":         "root",
		"INFLUX_PW":           "",
	}

	if instance.FilterType == "pipeId" {
		env["DATA_FILTER_ID_MAPPING"] = "pipeline_id"
	}

	labels := map[string]string{
		"io.rancher.container.pull_image":          "always",
		"io.rancher.scheduler.affinity:host_label": "role=worker",
	}
	request := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey)

	reqBody := &Request{
		Type:          "service",
		Name:          "kafka-influx-" + instance.ID.String(),
		StackId:       r.stackId,
		Scale:         1,
		StartOnCreate: true,
		LaunchConfig: LaunchConfig{
			ImageUuid:   "docker:fgseitsrancher.wifa.intern.uni-leipzig.de:5000/kafka-influx:unstable",
			Environment: env,
			Labels:      labels,
		},
	}
	resp, body, e := request.Post(r.url + "services").Send(reqBody).End()
	if resp.StatusCode != http.StatusCreated {
		fmt.Println("Something went totally wrong", resp)
	}
	if len(e) > 0 {
		fmt.Println("Something went wrong", e)
	}
	data := lib.ToJson(body)
	return data["id"].(string)
}

func (r Rancher) DeleteInstance(serviceId string) map[string]interface{} {
	request := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey)
	_, body, _ := request.Delete(r.url + "services/" + serviceId).End()
	return lib.ToJson(body)
}
