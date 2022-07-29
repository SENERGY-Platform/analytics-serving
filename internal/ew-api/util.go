package ew_api

import (
	"analytics-serving/internal/lib"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/segmentio/kafka-go"
	"log"
	"strings"
)

const (
	TypeDevice       = "deviceId"
	TypeAnalytics    = "operatorId"
	TypeImport       = "import_id"
	MappingData      = ":data"
	MappingExtra     = ":extra"
	IdentKeyDevice   = "device_id"
	IdentKeyService  = "service_id"
	IdentKeyPipeline = "pipeline_id"
	IdentKeyOperator = "operator_id"
	IdentKeyImport   = "import_id"
	InfluxDB         = "influxdb"
	TimescaleDB      = "timescaledb"
)

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
		dst := strings.Split(key, ":")
		mappings[dst[0]+mappingType] = val
	}
	return
}

func genMappings(mappings map[string]string, dataFields *string, tagFields *string) (err error) {
	if *dataFields != "" {
		err = addMappings(mappings, dataFields, MappingData)
	}
	if *tagFields != "" {
		err = addMappings(mappings, tagFields, MappingExtra)
	}
	return
}

func shortenId(longId string) (string, error) {
	parts := strings.Split(longId, ":")
	noPrefix := parts[len(parts)-1]
	noPrefix = strings.ReplaceAll(noPrefix, "-", "")
	bytes, err := hex.DecodeString(noPrefix)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func checkTopic(partitions *[]kafka.Partition, topic string) bool {
	for _, p := range *partitions {
		if p.Topic == topic {
			return true
		}
	}
	return false
}

func publishInstances(serving *lib.Serving, missingIds *[]string) (err error) {
	for _, id := range *missingIds {
		instances, _, errs := serving.GetInstances("", map[string][]string{"export_database_id": {id}}, true)
		if len(errs) > 0 {
			log.Println(errs)
			err = errors.New("getting instances failed")
			return
		}
		if len(instances) > 0 {
			for _, instance := range instances {
				log.Println("publishing instance '" + instance.ID.String() + "' to '" + instance.ExportDatabase.EwFilterTopic + "'")
				err = serving.CreateFromInstance(&instance)
				if err != nil {
					log.Println(err)
				}
			}
			log.Println("instances published")
		}
	}
	return
}
