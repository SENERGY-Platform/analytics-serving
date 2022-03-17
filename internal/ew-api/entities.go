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

const (
	MethodPut    = "put"
	MethodDelete = "delete"
)

type Message struct {
	Method    string `json:"method"`
	Payload   Filter `json:"payload"`
	Timestamp string `json:"timestamp"`
}

type Filter struct {
	Source      string            `json:"source,omitempty"`
	Identifiers []Identifier      `json:"identifiers,omitempty"`
	Mappings    map[string]string `json:"mappings,omitempty"`
	ExportID    string            `json:"export_id"`
	ExportArgs  interface{}       `json:"export_args,omitempty"`
}

type Identifier struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}
