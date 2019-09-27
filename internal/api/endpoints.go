/*
 * Copyright 2019 InfAI (CC SES)
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

package api

import (
	"analytics-serving/internal/lib"
	"encoding/json"
	"fmt"
	"net/http"

	"strings"

	"github.com/gorilla/mux"
)

type Endpoint struct {
	serving *lib.Serving
}

func NewEndpoint(driver lib.Driver) *Endpoint {
	ret := lib.NewServing(driver)
	return &Endpoint{ret}
}

func (e *Endpoint) getRootEndpoint(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(lib.Response{"OK"})
}

func (e *Endpoint) putNewServingInstance(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var servingReq lib.ServingRequest
	err := decoder.Decode(&servingReq)
	if err != nil {
		fmt.Println(err)
	}
	userId := req.Header.Get("X-UserId")
	if userId == "" {
		userId = "admin"
	}
	userId = strings.Replace(userId, "\"", "", -1)
	defer req.Body.Close()
	e.serving.CreateInstance(servingReq, userId)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(lib.Response{"OK"})
}

func (e *Endpoint) getServingInstance(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(e.serving.GetInstance(vars["id"]))
}

func (e *Endpoint) getServingInstances(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	args := req.URL.Query()
	userId := req.Header.Get("X-UserId")
	if userId == "" {
		userId = "admin"
	}
	userId = strings.Replace(userId, "\"", "", -1)
	json.NewEncoder(w).Encode(e.serving.GetInstances(userId, args))
}

func (e *Endpoint) deleteServingInstance(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(204)
	e.serving.DeleteInstance(vars["id"])
	json.NewEncoder(w).Encode(lib.Response{"OK"})
}