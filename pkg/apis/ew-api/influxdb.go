package ew_api

import (
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

func addInfluxDBCast(castMap map[string]string, fieldsMap map[string]string) (err error) {
	for key := range fieldsMap {
		dst := strings.Split(key, ":")
		castMap[dst[0]] = influxDBTypeMap[dst[1]]
	}
	return
}

func genInfluxExportArgs(args *InfluxDBExportArgs, dbName string, timePath string, timePrecision *string, dataFieldsMap map[string]string, tagFieldsMap map[string]string) (err error) {
	castMap := map[string]string{}
	if len(dataFieldsMap) > 0 {
		err = addInfluxDBCast(castMap, dataFieldsMap)
	}
	if len(tagFieldsMap) > 0 {
		err = addInfluxDBCast(castMap, tagFieldsMap)
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
