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
	"errors"
	"github.com/SENERGY-Platform/analytics-serving/internal/lib"
	"net/http"

	"github.com/parnurzeal/gorequest"
)

type Rancher struct {
	url       string
	accessKey string
	secretKey string
	stackId   string
}

func NewRancher(url string, accessKey string, secretKey string, stackId string) *Rancher {
	return &Rancher{url, accessKey, secretKey, stackId}
}

func (r Rancher) CreateInstance(instance *lib.Instance, dataFields string, tagFields string) (serviceId string, err error) {
	env := map[string]string{
		"KAFKA_GROUP_ID":      "transfer-" + instance.ApplicationId.String(),
		"KAFKA_BOOTSTRAP":     lib.GetEnv("KAFKA_BOOTSTRAP", "broker.kafka.rancher.internal:9092"),
		"KAFKA_TOPIC":         instance.Topic,
		"DATA_MEASUREMENT":    instance.Measurement,
		"DATA_FIELDS_MAPPING": dataFields,
		"DATA_TAGS_MAPPING":   tagFields,
		"DATA_TIME_MAPPING":   instance.TimePath,
		"DATA_FILTER_ID":      instance.Filter,
		"INFLUX_DB":           instance.Database,
		"INFLUX_HOST":         lib.GetEnv("INFLUX_DB_HOST", "influxdb"),
		"INFLUX_PORT":         lib.GetEnv("INFLUX_DB_PORT", "8086"),
		"INFLUX_USER":         lib.GetEnv("INFLUX_DB_USER", "root"),
		"INFLUX_PW":           lib.GetEnv("INFLUX_DB_PASSWORD", ""),
		"OFFSET_RESET":        instance.Offset,
	}

	if instance.TimePrecision != nil && *instance.TimePrecision != "" {
		env["TIME_PRECISION"] = *instance.TimePrecision
	}

	if instance.FilterType == "operatorId" {
		env["DATA_FILTER_ID_MAPPING"] = "operator_id"
	}

	if instance.FilterType == "import_id" {
		env["DATA_FILTER_ID_MAPPING"] = "import_id"
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
			ImageUuid: "docker:" + lib.GetEnv("TRANSFER_IMAGE",
				"fgseitsrancher.wifa.intern.uni-leipzig.de:5000/kafka-influx:unstable"),
			Environment: env,
			Labels:      labels,
		},
	}
	resp, body, e := request.Post(r.url + "services").Send(reqBody).End()
	if resp.StatusCode != http.StatusCreated {
		err = errors.New("could not create export")
		return
	}
	if len(e) > 0 {
		err = errors.New("could not create export")
		return
	}
	serviceId = lib.ToJson(body)["id"].(string)
	return
}

func (r Rancher) DeleteInstance(instance *lib.Instance) (err error) {
	request := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey)
	_, body, e := request.Delete(r.url + "services/" + instance.RancherServiceId).End()
	if len(e) > 0 {
		err = errors.New("could not delete export: " + body)
		return
	}
	return
}
