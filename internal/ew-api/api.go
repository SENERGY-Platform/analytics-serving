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
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
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
)

type InfluxDBExportArgs struct {
	DBName        string `json:"db_name"`
	TimeKey       string `json:"time_key,omitempty"`
	TimeFormat    string `json:"time_format,omitempty"`
	TimePrecision string `json:"time_precision,omitempty"`
}

type ExportWorker struct {
	kafkaProducer *kafka.Writer
}

func NewExportWorker(kafkaProducer *kafka.Writer) *ExportWorker {
	return &ExportWorker{kafkaProducer}
}

func (ew *ExportWorker) CreateInstance(instance *lib.Instance, dataFields string, tagFields string) (serviceId string, err error) {
	mappings := map[string]string{}
	if dataFields != "" {
		err = addMappings(mappings, &dataFields, MappingData)
		if err != nil {
			return "", err
		}
	}
	if tagFields != "" {
		err = addMappings(mappings, &tagFields, MappingExtra)
		if err != nil {
			return "", err
		}
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
	exportArgs := InfluxDBExportArgs{DBName: instance.Database}
	if instance.TimePath != "" {
		exportArgs.TimeKey = InfluxDBTimeKey
		mappings[InfluxDBTimeKey+MappingTypeString+MappingExtra] = instance.TimePath
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
			ID:          instance.Measurement,
			Args:        exportArgs,
		},
		Timestamp: time.Now().UTC().Unix(),
	}
	var jsonByte []byte
	jsonByte, err = json.Marshal(&message)
	if err != nil {
		return "", err
	}
	err = ew.kafkaProducer.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(instance.Measurement),
		Value: jsonByte,
	})
	return "", err
}

func (ew *ExportWorker) DeleteInstance(id string) (err error) {
	message := Message{
		Method: MethodDelete,
		Payload: Filter{
			ID: id,
		},
		Timestamp: time.Now().UTC().Unix(),
	}
	var jsonByte []byte
	jsonByte, err = json.Marshal(&message)
	if err != nil {
		return
	}
	err = ew.kafkaProducer.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(id),
		Value: jsonByte,
	})
	return
}

func addIdentifier(identifiers *[]Identifier, key string, value string) {
	*identifiers = append(*identifiers, Identifier{
		Key:   key,
		Value: value,
	})
}

func addMappings(mappings map[string]string, fields *string, mappingType string) (err error) {
	fieldsMap := map[string]string{}
	err = json.Unmarshal([]byte(*fields), &fieldsMap)
	if err != nil {
		return
	}
	for key, val := range fieldsMap {
		mappings[key+mappingType] = val
	}
	return
}
