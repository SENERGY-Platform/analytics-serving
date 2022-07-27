package ew_api

import (
	"analytics-serving/internal/lib"
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/segmentio/kafka-go"
	"log"
	"net"
	"strconv"
)

func checkTopic(partitions *[]kafka.Partition, topic string) bool {
	for _, p := range *partitions {
		if p.Topic == topic {
			return true
		}
	}
	return false
}

func createTopic(controllerConn *kafka.Conn, topic string) (err error) {
	log.Println("topic '" + topic + "' does not exist, creating ...")
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
	if err != nil {
		return
	}
	log.Println("topic '" + topic + "' created")
	return
}

func publishInstances(serving *lib.Serving, missing_topics *[]string, topicMap map[string]string) (err error) {
	instances, _, errs := serving.GetInstances("", map[string][]string{}, true)
	if len(errs) > 0 {
		log.Println(errs)
		err = errors.New("getting instances failed")
		return
	}
	if len(instances) > 0 {
		for _, topic := range *missing_topics {
			for _, instance := range instances {
				if topicMap[instance.DatabaseType] == topic {
					log.Println("publishing instance '" + instance.ID.String() + "' to '" + topic + "'")
					err = serving.CreateFromInstance(&instance)
					if err != nil {
						log.Println(err)
					}
				}
			}
		}
		log.Println("instances published")
	}
	return
}

func InitTopics(addr string, serving *lib.Serving) (err error) {
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
	var missing_topics []string
	for _, t := range topicMap {
		if !checkTopic(&partitions, t) {
			missing_topics = append(missing_topics, t)
		}
	}
	if len(missing_topics) > 0 {
		var controller kafka.Broker
		controller, err = conn.Controller()
		if err != nil {
			return
		}
		var controllerConn *kafka.Conn
		controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
		if err != nil {
			panic(err.Error())
		}
		defer func(controllerConn *kafka.Conn) {
			_ = controllerConn.Close()
		}(controllerConn)
		for _, t := range missing_topics {
			err = createTopic(controllerConn, t)
			if err != nil {
				return
			}
		}
		err = publishInstances(serving, &missing_topics, topicMap)
	}
	return
}
