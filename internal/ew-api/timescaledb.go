package ew_api

import (
	"errors"
	"strings"
)

var timescaleDBTypeMap = map[string][2]string{
	"string": {"text", "NULL"},
	"float":  {"double", "PRECISION NULL"},
	"int":    {"bigint", "NULL"},
	"bool":   {"bool", "NULL"},
}

type TimescaleDBExportArgs struct {
	TableName    string      `json:"table_name"`
	TableColumns [][3]string `json:"table_columns"`
	TimeColumn   string      `json:"time_column"`
	TimeFormat   string      `json:"time_format"`
}

func addTimescaleDBTimeMapping(mappings map[string]string, timePath string) (err error) {
	if timePath == "" {
		err = errors.New("column containing timestamps required")
		return
	} else {
		dst := strings.Split(timePath, ".")
		mappings[dst[len(dst)-1]+MappingData] = timePath
	}
	return
}

func addColumns(columns *[][3]string, fieldsMap map[string]string, timePath string) (err error) {
	tp := strings.Split(timePath, ".")
	*columns = append(*columns, [3]string{tp[len(tp)-1], "TIMESTAMP", "NOT NULL"})
	for key, _ := range fieldsMap {
		dst := strings.Split(key, ":")
		*columns = append(*columns, [3]string{dst[0], timescaleDBTypeMap[dst[1]][0], timescaleDBTypeMap[dst[1]][1]})
	}
	return
}

func genTimescaleDBExportArgs(args *TimescaleDBExportArgs, exportID string, dbName string, timePath string, timeFormat string, dataFieldsMap map[string]string) (err error) {
	if timePath == "" {
		err = errors.New("column containing timestamps required")
		return
	} else {
		dst := strings.Split(timePath, ".")
		args.TimeColumn = dst[len(dst)-1]
	}
	if timeFormat == "" {
		err = errors.New("timestamp format required")
		return
	} else {
		args.TimeFormat = timeFormat
	}
	var shortExportID string
	var shortDBName string
	shortExportID, err = shortenId(exportID)
	if err != nil {
		return
	}
	shortDBName, err = shortenId(dbName)
	if err != nil {
		return
	}
	args.TableName = "userid:" + shortDBName + "_export:" + shortExportID
	var columns [][3]string
	if len(dataFieldsMap) > 0 {
		err = addColumns(&columns, dataFieldsMap, timePath)
		if err != nil {
			return
		}
	}
	args.TableColumns = columns
	return
}
