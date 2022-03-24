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
	"log"
	"net"
	"strconv"
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
	serviceId = ""
	mappings := map[string]string{}
	err = genMappings(mappings, &dataFields, &tagFields, instance.TimePath)
	if err != nil {
		return
	}
	var identifiers []Identifier
	genIdentifiers(&identifiers, instance.FilterType, instance.Filter, instance.Topic)
	var exportArgs InfluxDBExportArgs
	genInfluxExportArgs(&exportArgs, instance.Database, instance.TimePath, instance.TimePrecision)
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
	err = ew.publish(&message, instance.Measurement)
	return
}

func (ew *ExportWorker) DeleteInstance(id string) (err error) {
	message := Message{
		Method: MethodDelete,
		Payload: Filter{
			ID: id,
		},
		Timestamp: time.Now().UTC().Unix(),
	}
	err = ew.publish(&message, id)
	return
}

func (ew *ExportWorker) publish(message *Message, key string) (err error) {
	var jsonByte []byte
	jsonByte, err = json.Marshal(message)
	if err != nil {
		return
	}
	err = ew.kafkaProducer.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(key),
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

func genIdentifiers(identifiers *[]Identifier, filterType string, filter string, topic string) {
	switch filterType {
	case TypeDevice:
		addIdentifier(identifiers, IdentKeyDevice, filter)
		addIdentifier(identifiers, IdentKeyService, strings.ReplaceAll(topic, "_", ":"))
	case TypeAnalytics:
		values := strings.Split(filter, ":")
		addIdentifier(identifiers, IdentKeyPipeline, values[0])
		addIdentifier(identifiers, IdentKeyOperator, values[1])
	case TypeImport:
		addIdentifier(identifiers, IdentKeyImport, filter)
	}
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

func genMappings(mappings map[string]string, dataFields *string, tagFields *string, timePath string) (err error) {
	if *dataFields != "" {
		err = addMappings(mappings, dataFields, MappingData)
	}
	if *tagFields != "" {
		err = addMappings(mappings, tagFields, MappingExtra)
	}
	if timePath != "" {
		mappings[InfluxDBTimeKey+MappingTypeString+MappingExtra] = timePath
	}
	return
}

func genInfluxExportArgs(args *InfluxDBExportArgs, dbName string, timePath string, timePrecision *string) {
	args.DBName = dbName
	if timePath != "" {
		args.TimeKey = InfluxDBTimeKey
	}
	if timePrecision != nil && *timePrecision != "" {
		args.TimePrecision = *timePrecision
	}
}

func InitTopic(addr string, topic string) (err error) {
	var conn *kafka.Conn
	conn, err = kafka.Dial("tcp", addr)
	if err != nil {
		return
	}
	defer func(conn *kafka.Conn) {
		_ = conn.Close()
	}(conn)
	var partitions []kafka.Partition
	partitions, err = conn.ReadPartitions()
	if err != nil {
		return
	}
	for _, p := range partitions {
		if p.Topic == topic {
			return
		}
	}
	var controller kafka.Broker
	controller, err = conn.Controller()
	if err != nil {
		return
	}
	log.Println("topic '" + topic + "' does not exist, creating ...")
	var controllerConn *kafka.Conn
	controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		panic(err.Error())
	}
	defer func(controllerConn *kafka.Conn) {
		_ = controllerConn.Close()
	}(controllerConn)
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
	err = controllerConn.CreateTopics(topicConfigs...)
	return
}
