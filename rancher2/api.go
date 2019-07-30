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

package rancher2

import (
	"analytics-serving/lib"
	"analytics-serving/model"
	"errors"
	"fmt"
	"net/http"

	"crypto/tls"

	"github.com/parnurzeal/gorequest"
)

var RANCHER2 *Rancher2

func Init() {
	r := NewRancher2(
		lib.GetEnv("RANCHER2_ENDPOINT", ""),
		lib.GetEnv("RANCHER2_ACCESS_KEY", ""),
		lib.GetEnv("RANCHER2_SECRET_KEY", ""),
		lib.GetEnv("RANCHER2_PROJECT_ID", ""),
		lib.GetEnv("RANCHER2_NAMESPACE_ID", ""),
	)
	RANCHER2 = r
}

type Rancher2 struct {
	url         string
	accessKey   string
	secretKey   string
	namespaceId string
	projectId   string
}

func NewRancher2(url string, accessKey string, secretKey string, namespaceId string, projectId string) *Rancher2 {
	return &Rancher2{url, accessKey, secretKey, namespaceId, projectId}
}

func (r *Rancher2) CreateInstance(instance *model.Instance, dataFields string) string {
	env := map[string]string{
		"KAFKA_GROUP_ID":      "transfer-" + instance.ID.String(),
		"KAFKA_BOOTSTRAP":     lib.GetEnv("KAFKA_BOOTSTRAP", "broker.kafka.rancher.internal:9092"),
		"KAFKA_TOPIC":         instance.Topic,
		"DATA_MEASUREMENT":    instance.Measurement,
		"DATA_FIELDS_MAPPING": dataFields,
		"DATA_TIME_MAPPING":   instance.TimePath,
		"DATA_FILTER_ID":      instance.Filter,
		"INFLUX_DB":           instance.Database,
		"INFLUX_HOST":         lib.GetEnv("INFLUX_DB_HOST", "influxdb"),
		"INFLUX_USER":         lib.GetEnv("INFLUX_DB_USER", "root"),
		"INFLUX_PW":           lib.GetEnv("INFLUX_DB_PASSWORD", ""),
	}

	if instance.FilterType == "pipeId" {
		env["DATA_FILTER_ID_MAPPING"] = "pipeline_id"
	}

	request := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey).TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	reqBody := &Request{
		Name:        "kafka-influx-" + instance.ID.String(),
		NamespaceId: r.namespaceId,
		Containers: []Container{{
			Image:       lib.GetEnv("TRANSFER_IMAGE", "fgseitsrancher.wifa.intern.uni-leipzig.de:5000/kafka-influx:unstable"),
			Name:        r.getInstanceName(instance.ID.String()),
			Environment: env,
		}},
		Labels:   map[string]string{"exportId": instance.ID.String()},
		Selector: Selector{MatchLabels: map[string]string{"exportId": instance.ID.String()}},
	}
	resp, body, e := request.Post(r.url + "projects/" + r.projectId + "/workloads").Send(reqBody).End()
	if resp.StatusCode != http.StatusCreated {
		fmt.Println("could not create export", body)
	}
	if len(e) > 0 {
		fmt.Println("something went wrong", e)
	}
	return ""
}

func (r *Rancher2) DeleteOperator(serviceId string) (err error) {
	request := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey).TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	resp, body, e := request.Delete(r.url + "projects/" + r.projectId + "/workloads/deployment:" +
		r.namespaceId + ":" + serviceId).End()
	if resp.StatusCode != http.StatusNoContent {
		err = errors.New("could not delete export: " + body)
		return
	}
	if len(e) > 0 {
		err = errors.New("something went wrong")
		return
	}
	return
}

func (r *Rancher2) getInstanceName(name string) string {
	return "kafka2influx-" + name
}
