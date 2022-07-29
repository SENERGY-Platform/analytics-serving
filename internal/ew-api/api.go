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
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/segmentio/kafka-go"
	"log"
	"time"
)

type ExportWorker struct {
	kafkaProducer       *kafka.Writer
	kafkaConn           *kafka.Conn
	kafkaControllerConn *kafka.Conn
}

func NewExportWorker(kafkaProducer *kafka.Writer, kafkaConn *kafka.Conn, kafkaControllerConn *kafka.Conn) *ExportWorker {
	return &ExportWorker{kafkaProducer, kafkaConn, kafkaControllerConn}
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
	switch instance.ExportDatabase.Type {
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
	err = ew.publish(&message, instance.ID.String(), instance.ExportDatabase.EwFilterTopic)
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
	err = ew.publish(&message, instance.ID.String(), instance.ExportDatabase.EwFilterTopic)
	return
}

func (ew *ExportWorker) CreateFilterTopic(topic string, checkExists bool) (err error) {
	if checkExists {
		var partitions []kafka.Partition
		partitions, err = ew.kafkaConn.ReadPartitions()
		if err != nil {
			return
		}
		if checkTopic(&partitions, topic) {
			return
		}
	}
	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 2,
			ConfigEntries: []kafka.ConfigEntry{
				{
					ConfigName:  "retention.ms",
					ConfigValue: "-1",
				},
				{
					ConfigName:  "retention.bytes",
					ConfigValue: "-1",
				},
				{
					ConfigName:  "cleanup.policy",
					ConfigValue: "compact",
				},
				{
					ConfigName:  "delete.retention.ms",
					ConfigValue: "86400000",
				},
				{
					ConfigName:  "segment.ms",
					ConfigValue: "604800000",
				},
				{
					ConfigName:  "min.cleanable.dirty.ratio",
					ConfigValue: "0.1",
				},
			},
		},
	}
	err = ew.kafkaControllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		return
	}
	log.Println("topic '" + topic + "' created")
	return
}

func (ew *ExportWorker) InitFilterTopics(serving *lib.Serving) (err error) {
	var databases []lib.ExportDatabase
	errs := lib.DB.Find(&databases).GetErrors()
	if len(errs) > 0 {
		for _, e := range errs {
			if gorm.IsRecordNotFoundError(e) {
				return
			}
		}
		err = errors.New("retrieving export-databases failed - " + fmt.Sprint(errs))
		return
	}
	var partitions []kafka.Partition
	partitions, err = ew.kafkaConn.ReadPartitions()
	if err != nil {
		return
	}
	var missingIds []string
	var missingTopics []string
	for _, database := range databases {
		if !checkTopic(&partitions, database.EwFilterTopic) {
			missingIds = append(missingIds, database.ID)
			if !stringInSlice(&missingTopics, database.EwFilterTopic) {
				missingTopics = append(missingTopics, database.EwFilterTopic)
			}
		}
	}
	if len(missingIds) > 0 {
		for _, topic := range missingTopics {
			err = ew.CreateFilterTopic(topic, false)
			if err != nil {
				return
			}
		}
		err = publishInstances(serving, &missingIds)
	}
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
