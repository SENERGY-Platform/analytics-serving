/*
 * Copyright 2022 InfAI (CC SES)
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

package ew_api

import (
	"analytics-serving/internal/lib"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	TypeDevice        = "deviceId"
	TypeAnalytics     = "operatorId"
	TypeImport        = "import_id"
	InfluxDBTimeKey   = "time"
	MappingData       = ":data"
	MappingExtra      = ":extra"
	MappingTypeString = ":string"
	IdentKeyDevice    = "device_id"
	IdentKeyService   = "service_id"
	IdentKeyPipeline  = "pipeline_id"
	IdentKeyOperator  = "operator_id"
	IdentKeyImport    = "import_id"
	TimeFormat        = "2006-01-02T15:04:05.999Z"
)

type InfluxDBExportArgs struct {
	DBName        string `json:"db_name"`
	TimeKey       string `json:"time_key"`
	TimeFormat    string `json:"time_format,omitempty"`
	TimePrecision string `json:"time_precision,omitempty"`
}

type ExportWorker struct {
	filterTopic string
}

func NewExportWorker(filterTopic string) *ExportWorker {
	return &ExportWorker{filterTopic}
}

func (ew *ExportWorker) CreateInstance(instance *lib.Instance, dataFields string, tagFields string) (serviceId string, err error) {
	mappings := map[string]string{}
	if dataFields != "" {
		dataMappings := map[string]string{}
		err := json.Unmarshal([]byte(dataFields), &dataMappings)
		if err != nil {
			return "", err
		}
		for key, val := range dataMappings {
			mappings[key+MappingData] = val
		}
	}
	if tagFields != "" {
		extraMappings := map[string]string{}
		err := json.Unmarshal([]byte(tagFields), &extraMappings)
		if err != nil {
			return "", err
		}
		for key, val := range extraMappings {
			mappings[key+MappingExtra] = val
		}
	}
	if instance.TimePath != "" {
		mappings[InfluxDBTimeKey+MappingTypeString+MappingExtra] = instance.TimePath
	}
	var identifiers []Identifier
	switch instance.FilterType {
	case TypeDevice:
		addIdentifier(&identifiers, IdentKeyDevice, instance.Filter)
		addIdentifier(&identifiers, IdentKeyService, strings.ReplaceAll(instance.Topic, "_", ":"))
	case TypeAnalytics:
		values := strings.Split(instance.Filter, ":")
		addIdentifier(&identifiers, IdentKeyPipeline, values[0])
		addIdentifier(&identifiers, IdentKeyOperator, values[1])
	case TypeImport:
		addIdentifier(&identifiers, IdentKeyImport, instance.Filter)
	}
	exportArgs := InfluxDBExportArgs{
		DBName:  instance.Database,
		TimeKey: InfluxDBTimeKey,
	}
	if instance.TimePrecision != nil && *instance.TimePrecision != "" {
		exportArgs.TimePrecision = *instance.TimePrecision
	}
	message := Message{
		Method: MethodPut,
		Payload: Filter{
			Source:      instance.Topic,
			Identifiers: identifiers,
			Mappings:    mappings,
			ExportID:    instance.Measurement,
			ExportArgs:  exportArgs,
		},
		Timestamp: time.Now().Format(TimeFormat),
	}
	jsonByte, err := json.Marshal(&message)
	if err != nil {
		return "", err
	}
	fmt.Println(string(jsonByte))
	return "", err
}

func (ew *ExportWorker) DeleteInstance(id string) (err error) {
	//TODO implement me
	panic("implement me")
}

func addIdentifier(identifiers *[]Identifier, key string, value string) {
	*identifiers = append(*identifiers, Identifier{
		Key:   key,
		Value: value,
	})
}
