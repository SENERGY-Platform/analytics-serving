/*
 * Copyright 2018 InfAI (CC SES)
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

package lib

type Response struct {
	Message string `json:"message,omitempty" validate:"required"`
}

type ServingRequest struct {
	FilterType       string                `json:"FilterType,omitempty" validate:"required"`
	Filter           string                `json:"Filter,omitempty" validate:"required"`
	Name             string                `json:"Name,omitempty" validate:"required"`
	EntityName       string                `json:"EntityName,omitempty" validate:"required"`
	ServiceName      string                `json:"ServiceName,omitempty" validate:"required"`
	Description      string                `json:"Description,omitempty"`
	Topic            string                `json:"Topic,omitempty" validate:"required"`
	TimePath         string                `json:"TimePath,omitempty"`
	TimePrecision    string                `json:"TimePrecision,omitempty"`
	Generated        bool                  `json:"generated,omitempty"`
	Offset           string                `json:"Offset,omitempty" validate:"required"`
	ForceUpdate      bool                  `json:"ForceUpdate,omitempty"`
	Values           []ServingRequestValue `json:"Values,omitempty"`
	DatabaseType     string                `json:"DatabaseType,omitempty"`
	ExportDatabaseID string                `json:"ExportDatabaseID,omitempty"`
	TimestampFormat  string                `json:"TimestampFormat,omitempty"`
}

type ServingRequestValue struct {
	Name string `json:"Name,omitempty"`
	Type string `json:"Type,omitempty"`
	Path string `json:"Path,omitempty"`
	Tag  bool   `json:"Tag"`
}

type InstancesResponse struct {
	Total     int64     `json:"total,omitempty"`
	Count     int       `json:"count,omitempty"`
	Instances Instances `json:"instances,omitempty"`
}

type Claims struct {
	Sub         string              `json:"sub,omitempty"`
	RealmAccess map[string][]string `json:"realm_access,omitempty"`
}

type ExportDatabaseRequest struct {
	Name          string `json:"name" validate:"required"`
	Description   string `json:"description"`
	Type          string `json:"type" validate:"required"`
	External      bool   `json:"external"`
	Url           string `json:"url" validate:"required"`
	EwFilterTopic string `json:"ew_filter_topic" validate:"required"`
	Public        bool   `json:"public"`
}

func (c Claims) Valid() error {
	return nil
}

func (c Claims) IsAdmin() bool {
	for _, b := range c.RealmAccess["roles"] {
		if b == "admin" {
			return true
		}
	}
	return false
}
