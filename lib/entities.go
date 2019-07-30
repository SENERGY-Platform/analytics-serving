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
	FilterType  string  `json:"FilterType,omitempty"`
	Filter      string  `json:"Filter,omitempty"`
	Name        string  `json:"Name,omitempty"`
	EntityName  string  `json:"EntityName,omitempty"`
	ServiceName string  `json:"ServiceName,omitempty"`
	Description string  `json:"Description,omitempty"`
	Topic       string  `json:"Topic,omitempty"`
	TimePath    string  `json:"TimePath,omitempty"`
	Values      []Value `json:"Values,omitempty"`
}

type Value struct {
	Name string `json:"Name,omitempty"`
	Type string `json:"Type,omitempty"`
	Path string `json:"Path,omitempty"`
}
