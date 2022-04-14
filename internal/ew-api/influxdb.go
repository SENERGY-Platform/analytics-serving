package ew_api

import (
	"encoding/json"
	"strings"
)

const (
	InfluxDBTimeKey = "time"
)

var influxDBTypeMap = map[string]string{
	"string":      ":string",
	"float":       ":number",
	"int":         ":integer",
	"bool":        ":boolean",
	"string_json": "object:string",
}

type InfluxDBExportArgs struct {
	DBName        string            `json:"db_name"`
	TypeCasts     map[string]string `json:"type_casts,omitempty"`
	TimeKey       string            `json:"time_key,omitempty"`
	TimeFormat    string            `json:"time_format,omitempty"`
	TimePrecision string            `json:"time_precision,omitempty"`
}

func addInfluxDBTimeMapping(mappings map[string]string, timePath string) {
	if timePath != "" {
		mappings[InfluxDBTimeKey+MappingExtra] = timePath
	}
}

func addInfluxDBCast(castMap map[string]string, fields *string) (err error) {
	fieldsMap := map[string]string{}
	err = json.Unmarshal([]byte(*fields), &fieldsMap)
	if err != nil {
		return
	}
	for key, _ := range fieldsMap {
		dst := strings.Split(key, ":")
		castMap[dst[0]] = influxDBTypeMap[dst[1]]
	}
	return
}

func genInfluxExportArgs(args *InfluxDBExportArgs, dbName string, timePath string, timePrecision *string, dataFields *string, tagFields *string) (err error) {
	castMap := map[string]string{}
	if *dataFields != "" {
		err = addInfluxDBCast(castMap, dataFields)
	}
	if *tagFields != "" {
		err = addInfluxDBCast(castMap, tagFields)
	}
	args.TypeCasts = castMap
	args.DBName = dbName
	if timePath != "" {
		args.TimeKey = InfluxDBTimeKey
	}
	if timePrecision != nil && *timePrecision != "" {
		args.TimePrecision = *timePrecision
	}
	return
}
