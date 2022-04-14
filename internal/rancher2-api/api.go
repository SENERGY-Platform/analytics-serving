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

package rancher2_api

import (
	"analytics-serving/internal/lib"
	"errors"
	"net/http"

	"crypto/tls"

	"github.com/parnurzeal/gorequest"
)

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

func (r *Rancher2) CreateInstance(instance *lib.Instance, dataFields string, tagFields string) (serviceId string, err error) {
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

	var r2Env []Env
	for k, v := range env {
		r2Env = append(r2Env, Env{
			Name:  k,
			Value: v,
		})
	}

	request := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey).TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	reqBody := &Request{
		Name:        r.getInstanceName(instance.ID.String()),
		NamespaceId: r.namespaceId,
		Containers: []Container{{
			Image:           lib.GetEnv("TRANSFER_IMAGE", "fgseitsrancher.wifa.intern.uni-leipzig.de:5000/kafka-influx:unstable"),
			Name:            "kafka2influx",
			Env:             r2Env,
			ImagePullPolicy: "Always",
			Resources:       Resources{Limits: Limits{Cpu: "0.1"}},
		}},
		Scheduling: Scheduling{Scheduler: "default-scheduler", Node: Node{RequireAll: []string{"role=worker"}}},
		Labels:     map[string]string{"exportId": instance.ID.String()},
		Selector:   Selector{MatchLabels: map[string]string{"exportId": instance.ID.String()}},
	}
	resp, _, e := request.Post(r.url + "projects/" + r.projectId + "/workloads").Send(reqBody).End()
	if len(e) > 0 {
		err = errors.New("could not create export")
		return
	}
	if resp.StatusCode != http.StatusCreated {
		err = errors.New("could not create export")
		return
	}
	return "", err
}

func (r *Rancher2) DeleteInstance(instance *lib.Instance) (err error) {
	request := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey).TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	resp, body, e := request.Delete(r.url + "projects/" + r.projectId + "/workloads/deployment:" +
		r.namespaceId + ":" + r.getInstanceName(instance.ID.String())).End()
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
