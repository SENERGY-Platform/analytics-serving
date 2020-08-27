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
	Message string `json:"message,omitempty"`
}

type ServingRequest struct {
	FilterType    string                `json:"FilterType,omitempty" validate:"required"`
	Filter        string                `json:"Filter,omitempty" validate:"required"`
	Name          string                `json:"Name,omitempty" validate:"required"`
	EntityName    string                `json:"EntityName,omitempty" validate:"required"`
	ServiceName   string                `json:"ServiceName,omitempty" validate:"required"`
	Description   string                `json:"Description,omitempty"`
	Topic         string                `json:"Topic,omitempty" validate:"required"`
	TimePath      string                `json:"TimePath,omitempty"`
	TimePrecision string                `json:"TimePrecision,omitempty"`
	Generated     bool                  `json:"generated,omitempty"`
	Offset        string                `json:"Offset,omitempty" validate:"required"`
	Values        []ServingRequestValue `json:"Values,omitempty"`
}

type ServingRequestValue struct {
	Name string `json:"Name,omitempty"`
	Type string `json:"Type,omitempty"`
	Path string `json:"Path,omitempty"`
}

type Claims struct {
	Sub         string              `json:"sub,omitempty"`
	RealmAccess map[string][]string `json:"realm_access,omitempty"`
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
