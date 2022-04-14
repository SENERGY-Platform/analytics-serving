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
	"errors"
	"github.com/segmentio/kafka-go"
	"time"
)

type ExportWorker struct {
	kafkaProducer *kafka.Writer
	topicMap      map[string]string
}

func NewExportWorker(kafkaProducer *kafka.Writer, topicMap map[string]string) *ExportWorker {
	return &ExportWorker{kafkaProducer, topicMap}
}

func (ew *ExportWorker) CreateInstance(instance *lib.Instance, dataFields string, tagFields string) (serviceId string, err error) {
	serviceId = ""
	filter := Filter{
		Source:      instance.Topic,
		Identifiers: []Identifier{},
		Mappings:    map[string]string{},
		ID:          instance.ID.String(),
	}
	genIdentifiers(&filter.Identifiers, instance.FilterType, instance.Filter, instance.Topic)
	err = genMappings(filter.Mappings, &dataFields, &tagFields)
	if err != nil {
		return
	}
	switch instance.DatabaseType {
	case InfluxDB:
		addInfluxDBTimeMapping(filter.Mappings, instance.TimePath)
		influxDBExportArgs := InfluxDBExportArgs{}
		err = genInfluxExportArgs(&influxDBExportArgs, instance.Database, instance.TimePath, instance.TimePrecision, &dataFields, &tagFields)
		if err != nil {
			return
		}
		filter.Args = influxDBExportArgs
	case TimescaleDB:
		err = addTimescaleDBTimeMapping(filter.Mappings, instance.TimePath)
		if err != nil {
			return
		}
		timescaleDBExportArgs := TimescaleDBExportArgs{}
		err = genTimescaleDBExportArgs(&timescaleDBExportArgs, instance.ID.String(), instance.Database, instance.TimePath, instance.TimestampFormat, &dataFields)
		if err != nil {
			return
		}
		filter.Args = timescaleDBExportArgs
	default:
		err = errors.New("unknown or missing database type")
		return
	}
	message := Message{
		Method:    MethodPut,
		Payload:   filter,
		Timestamp: time.Now().UTC().Unix(),
	}
	err = ew.publish(&message, instance.ID.String(), ew.topicMap[instance.DatabaseType])
	return
}

func (ew *ExportWorker) DeleteInstance(instance *lib.Instance) (err error) {
	message := Message{
		Method: MethodDelete,
		Payload: Filter{
			ID: instance.ID.String(),
		},
		Timestamp: time.Now().UTC().Unix(),
	}
	err = ew.publish(&message, instance.ID.String(), ew.topicMap[instance.DatabaseType])
	return
}

func (ew *ExportWorker) publish(message *Message, key string, topic string) (err error) {
	var jsonByte []byte
	jsonByte, err = json.Marshal(message)
	if err != nil {
		return
	}
	err = ew.kafkaProducer.WriteMessages(context.Background(), kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: jsonByte,
	})
	return
}
