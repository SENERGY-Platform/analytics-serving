/*
 * Copyright 2026 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"slices"
	"strings"

	"github.com/jinzhu/gorm"
)

// QueryError reports a query parameter of a list request that cannot be applied; the handlers answer it with 400.
type QueryError struct {
	Parameter string
	Message   string
}

func (e *QueryError) Error() string {
	return "invalid query parameter " + e.Parameter + ": " + e.Message
}

var instanceOrderColumns = []string{"id", "name", "description", "entity_name", "service_name", "topic",
	"application_id", "database", "measurement", "filter", "filter_type", "time_path", "time_precision", "user_id",
	"generated", "rancher_service_id", "offset", "export_database_id", "timestamp_format", "timestamp_unique",
	"created_at", "updated_at"}

var exportDatabaseOrderColumns = []string{"id", "name", "description", "type", "deployment", "url",
	"ew_filter_topic", "user_id", "public"}

type order struct {
	column    string
	direction string
}

// parseOrder validates "column:direction" from the order query parameter, since gorm writes an order string
// into the SQL verbatim. The direction is returned as a constant, never as the caller's spelling.
func parseOrder(value string, allowed []string) (order, error) {
	column, direction, found := strings.Cut(value, ":")
	if !found || !slices.Contains(allowed, column) {
		return order{}, &QueryError{Parameter: "order", Message: "must be <column>:asc or <column>:desc with one of the columns " + strings.Join(allowed, ", ")}
	}
	switch strings.ToLower(direction) {
	case "asc":
		return order{column: column, direction: "asc"}, nil
	case "desc":
		return order{column: column, direction: "desc"}, nil
	}
	return order{}, &QueryError{Parameter: "order", Message: "direction must be asc or desc"}
}

// parseOrderArg reads the order query parameter; ok is false if it is absent.
func parseOrderArg(args map[string][]string, allowed []string) (o order, ok bool, err error) {
	values, present := args["order"]
	if !present || len(values) == 0 {
		return order{}, false, nil
	}
	o, err = parseOrder(values[0], allowed)
	return o, err == nil, err
}

// clause quotes the column, because some allowed names such as offset are reserved words.
func (o order) clause(db *gorm.DB) string {
	return db.Dialect().Quote(o.column) + " " + o.direction
}
