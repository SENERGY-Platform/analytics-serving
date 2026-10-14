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

package api

import (
	"reflect"
	"strings"
	"testing"

	"github.com/SENERGY-Platform/analytics-serving/lib"
	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
)

func validServingRequest() lib.ServingRequest {
	return lib.ServingRequest{
		FilterType:  "deviceId",
		Filter:      "device1",
		Name:        "name",
		EntityName:  "entity",
		ServiceName: "service",
		Topic:       "topic",
		Offset:      "largest",
	}
}

func TestValidateInputsServingRequest(t *testing.T) {
	util.InitStructLogger("error")
	// "ä" is two bytes in UTF-8, so the 255 case also shows that characters are counted, not bytes.
	chars255 := strings.Repeat("ä", 255)
	chars256 := strings.Repeat("ä", 256)

	cases := []struct {
		name   string
		modify func(r *lib.ServingRequest)
		want   map[string][]string
	}{
		{"valid", func(r *lib.ServingRequest) {}, nil},
		{"every allowed filter type: operatorId", func(r *lib.ServingRequest) { r.FilterType = "operatorId" }, nil},
		{"every allowed filter type: import_id", func(r *lib.ServingRequest) { r.FilterType = "import_id" }, nil},
		{"unknown filter type", func(r *lib.ServingRequest) { r.FilterType = "foo" },
			map[string][]string{"FilterType": {"The field 'FilterType' must be one of: deviceId, operatorId, import_id"}}},
		{"filter type differs in case", func(r *lib.ServingRequest) { r.FilterType = "DeviceId" },
			map[string][]string{"FilterType": {"The field 'FilterType' must be one of: deviceId, operatorId, import_id"}}},
		{"missing filter type is reported as required only", func(r *lib.ServingRequest) { r.FilterType = "" },
			map[string][]string{"FilterType": {"The field 'FilterType' is required"}}},
		{"255 multibyte characters", func(r *lib.ServingRequest) {
			r.Description, r.Name, r.TimePath, r.TimestampFormat = chars255, chars255, chars255, chars255
			r.Values = []lib.ServingRequestValue{{Name: chars255, Type: chars255, Path: chars255}}
		}, nil},
		{"256 characters", func(r *lib.ServingRequest) { r.Description = chars256 },
			map[string][]string{"Description": {"The field 'Description' must not have more than 255 characters"}}},
		{"256 characters in every limited field", func(r *lib.ServingRequest) {
			r.Filter, r.Name, r.EntityName, r.ServiceName, r.Description, r.Topic = chars256, chars256, chars256, chars256, chars256, chars256
			r.TimePath, r.TimePrecision, r.Offset, r.ExportDatabaseID, r.TimestampFormat = chars256, chars256, chars256, chars256, chars256
		}, map[string][]string{
			"Filter":           {"The field 'Filter' must not have more than 255 characters"},
			"Name":             {"The field 'Name' must not have more than 255 characters"},
			"EntityName":       {"The field 'EntityName' must not have more than 255 characters"},
			"ServiceName":      {"The field 'ServiceName' must not have more than 255 characters"},
			"Description":      {"The field 'Description' must not have more than 255 characters"},
			"Topic":            {"The field 'Topic' must not have more than 255 characters"},
			"TimePath":         {"The field 'TimePath' must not have more than 255 characters"},
			"TimePrecision":    {"The field 'TimePrecision' must not have more than 255 characters"},
			"Offset":           {"The field 'Offset' must not have more than 255 characters"},
			"ExportDatabaseID": {"The field 'ExportDatabaseID' must not have more than 255 characters"},
			"TimestampFormat":  {"The field 'TimestampFormat' must not have more than 255 characters"},
		}},
		{"value fields are named by their path", func(r *lib.ServingRequest) {
			r.Values = []lib.ServingRequestValue{{Name: "ok"}, {Name: chars256, Type: chars256, Path: chars256}}
		}, map[string][]string{
			"Values[1].Name": {"The field 'Values[1].Name' must not have more than 255 characters"},
			"Values[1].Type": {"The field 'Values[1].Type' must not have more than 255 characters"},
			"Values[1].Path": {"The field 'Values[1].Path' must not have more than 255 characters"},
		}},
		{"missing required field keeps its plain name", func(r *lib.ServingRequest) { r.ServiceName = "" },
			map[string][]string{"ServiceName": {"The field 'ServiceName' is required"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := validServingRequest()
			tc.modify(&request)
			validated, errs := ValidateInputs(request)
			if validated != (tc.want == nil) {
				t.Fatalf("validated = %v, errors = %v", validated, errs)
			}
			if tc.want != nil && !reflect.DeepEqual(errs, tc.want) {
				t.Errorf("errors = %v, want %v", errs, tc.want)
			}
		})
	}
}
