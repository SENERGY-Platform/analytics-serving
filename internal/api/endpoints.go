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
	"github.com/golang-jwt/jwt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type Endpoint struct {
	serving *lib.Serving
}

func NewEndpoint(driver lib.Driver, permissionService lib.PermissionApiService, pipelineService lib.PipelineApiService, importDeployService lib.ImportDeployService) *Endpoint {
	ret := lib.NewServing(driver, permissionService, pipelineService, importDeployService)
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
		log.Println(err)
	}
	defer req.Body.Close()

	validated, errors := ValidateInputs(servingReq)

	if !validated {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]map[string][]string{"validationErrors": errors})
	} else {
		userId, _ := getUserInfo(req)
		instance, err := e.serving.CreateInstance(servingReq, userId, req.Header.Get("Authorization"))
		if err != nil {
			log.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(instance)
		}
	}
}

func (e *Endpoint) putNewServingInstance(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var servingReq lib.ServingRequest
	err := decoder.Decode(&servingReq)
	if err != nil {
		log.Println(err)
	}
	defer req.Body.Close()

	validated, errors := ValidateInputs(servingReq)
	w.Header().Set("Content-Type", "application/json")
	if !validated {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]map[string][]string{"validationErrors": errors})
	} else {
		userId, _ := getUserInfo(req)
		vars := mux.Vars(req)
		instance, errors := e.serving.UpdateInstance(vars["id"], userId, servingReq, req.Header.Get("Authorization"))
		if len(errors) > 0 {
			for _, err := range errors {
				log.Println(err)
			}
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(instance)
		}
	}
}

func (e *Endpoint) getServingInstance(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Set("Content-Type", "application/json")
	userId, _ := getUserInfo(req)
	instance, errors := e.serving.GetInstance(vars["id"], userId)
	if len(errors) > 0 {
		for _, err := range errors {
			log.Println(err)
		}
		w.WriteHeader(500)
	} else {
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(instance)
	}
}

func (e *Endpoint) getServingInstances(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	args := req.URL.Query()
	userId, _ := getUserInfo(req)
	instances, total, errors := e.serving.GetInstancesForUser(userId, args)
	if len(errors) > 0 {
		log.Println(errors)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(lib.InstancesResponse{
			Total:     total,
			Count:     len(instances),
			Instances: instances,
		})
	}
}

func (e *Endpoint) deleteServingInstance(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	userId, _ := getUserInfo(req)
	deleted, errors := e.serving.DeleteInstanceForUser(vars["id"], userId)
	w.Header().Set("Content-Type", "application/json")
	if len(errors) > 0 && deleted == false {
		for _, err := range errors {
			log.Println(err)
		}
		w.WriteHeader(500)
	} else {
		w.WriteHeader(204)
	}
}

func (e *Endpoint) deleteServingInstances(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var requestBody []string
	err := decoder.Decode(&requestBody)
	if err != nil {
		log.Println(err)
	}
	defer req.Body.Close()
	userId, _ := getUserInfo(req)
	deleted, errors := e.serving.DeleteInstancesForUser(requestBody, userId)
	w.Header().Set("Content-Type", "application/json")
	if len(errors) > 0 && len(deleted) < 1 {
		for _, err := range errors {
			log.Println(err)
		}
		w.WriteHeader(500)
	} else if len(errors) > 0 && len(deleted) > 0 {
		w.WriteHeader(http.StatusMultiStatus)
		_ = json.NewEncoder(w).Encode(lib.Response{
			Message: "Could not delete all exports. Exports deleted: " + strings.Join(deleted, ","),
		})
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (e *Endpoint) getServingInstancesAdmin(w http.ResponseWriter, req *http.Request) {
	args := req.URL.Query()
	userId, admin := getUserInfo(req)
	w.Header().Set("Content-Type", "application/json")
	if admin {
		instances, _, errors := e.serving.GetInstances(userId, args, admin)
		if len(errors) > 0 {
			log.Println(errors)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(instances)
		}
	} else {
		w.WriteHeader(403)
		_ = json.NewEncoder(w).Encode("Forbidden")
	}
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
			log.Println(err)
		}
		w.WriteHeader(500)
	} else {
		w.WriteHeader(204)
	}
}

func getUserInfo(req *http.Request) (userId string, admin bool) {
	//userId = req.Header.Get("X-UserId")
	if userId == "" {
		if len(req.Header.Get("Authorization")) > 0 {
			_, claims := parseJWTToken(req.Header.Get("Authorization")[7:])
			userId = claims.Sub
			admin = claims.IsAdmin()
			if userId == "" {
				userId = "dummy"
				admin = false
			}
		}
	}
	return
}

func parseJWTToken(encodedToken string) (token *jwt.Token, claims lib.Claims) {
	token, _ = jwt.ParseWithClaims(encodedToken, &claims, nil)
	return
}
