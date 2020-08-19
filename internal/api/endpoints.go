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
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"net/http"
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

func (e *Endpoint) postNewServingInstance(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var servingReq lib.ServingRequest
	err := decoder.Decode(&servingReq)
	if err != nil {
		fmt.Println(err)
	}
	defer req.Body.Close()

	validated, errors := ValidateInputs(servingReq)

	if !validated {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]map[string][]string{"validationErrors": errors})
	} else {
		userId, _ := getUserInfo(req)
		instance := e.serving.CreateInstance(servingReq, userId)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(instance)
	}
}

func (e *Endpoint) getServingInstance(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Set("Content-Type", "application/json")
	userId, _ := getUserInfo(req)
	instance, errors := e.serving.GetInstance(vars["id"], userId)
	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Println(err)
		}
		w.WriteHeader(404)
	} else {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(instance)
	}
}

func (e *Endpoint) getServingInstances(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	args := req.URL.Query()
	userId, _ := getUserInfo(req)
	json.NewEncoder(w).Encode(e.serving.GetInstancesForUser(userId, args))
}

func (e *Endpoint) deleteServingInstance(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	userId, _ := getUserInfo(req)
	deleted, errors := e.serving.DeleteInstanceForUser(vars["id"], userId)
	w.Header().Set("Content-Type", "application/json")
	if len(errors) > 0 && deleted == false {
		w.WriteHeader(404)
	} else if len(errors) > 0 {
		for _, err := range errors {
			fmt.Println(err)
		}
		w.WriteHeader(204)
	} else {
		w.WriteHeader(204)
	}
}

func (e *Endpoint) getServingInstancesAdmin(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	args := req.URL.Query()
	userId, admin := getUserInfo(req)
	json.NewEncoder(w).Encode(e.serving.GetInstances(userId, args, admin))
}

func (e *Endpoint) deleteServingInstanceAdmin(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	userId, admin := getUserInfo(req)
	deleted, errors := e.serving.DeleteInstance(vars["id"], userId, admin)
	w.Header().Set("Content-Type", "application/json")
	if len(errors) > 0 && deleted == false {
		w.WriteHeader(404)
	} else if len(errors) > 0 {
		for _, err := range errors {
			fmt.Println(err)
		}
		w.WriteHeader(204)
	} else {
		w.WriteHeader(204)
	}
}

func getUserInfo(req *http.Request) (userId string, admin bool) {
	//userId = req.Header.Get("X-UserId")
	if userId == "" {
		_, claims := parseJWTToken(req.Header.Get("Authorization")[7:])
		userId = claims.Sub
		admin = claims.IsAdmin()
		if userId == "" {
			userId = "admin"
			admin = false
		}
	}
	return
}

func parseJWTToken(encodedToken string) (token *jwt.Token, claims lib.Claims) {
	token, _ = jwt.ParseWithClaims(encodedToken, &claims, nil)
	return
}
