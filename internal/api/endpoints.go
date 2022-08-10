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
	"github.com/golang-jwt/jwt"
	"github.com/jinzhu/gorm"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type Endpoint struct {
	serving *lib.Serving
}

func NewEndpoint(serving *lib.Serving) *Endpoint {
	return &Endpoint{serving: serving}
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
	if len(errors) == 0 && deleted == true {
		w.WriteHeader(http.StatusNoContent)
	} else if len(errors) == 0 && deleted == false {
		w.WriteHeader(http.StatusNotFound)
	} else if len(errors) > 0 && deleted == true {
		w.WriteHeader(http.StatusMultiStatus)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	if len(errors) > 0 {
		err := json.NewEncoder(w).Encode(map[string]string{"deleted": fmt.Sprint(deleted), "error": fmt.Sprint(errors)})
		if err != nil {
			log.Println(err)
		}
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
	if len(errors) == 0 && deleted == true {
		w.WriteHeader(http.StatusNoContent)
	} else if len(errors) == 0 && deleted == false {
		w.WriteHeader(http.StatusNotFound)
	} else if len(errors) > 0 && deleted == true {
		w.WriteHeader(http.StatusMultiStatus)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	if len(errors) > 0 {
		err := json.NewEncoder(w).Encode(map[string]string{"deleted": fmt.Sprint(deleted), "error": fmt.Sprint(errors)})
		if err != nil {
			log.Println(err)
		}
	}
}

func (e *Endpoint) readExportDatabases(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userId, _ := getUserInfo(req)
	if userId == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(map[string]string{"error": "missing user id"})
		if err != nil {
			log.Println(err)
		}
	} else {
		args := req.URL.Query()
		databases, errs := e.serving.GetExportDatabases(userId, args)
		if errs != nil && len(errs) > 0 {
			w.WriteHeader(http.StatusInternalServerError)
			err := json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprint(errs)})
			if err != nil {
				log.Println(err)
			}
		} else {
			w.WriteHeader(http.StatusOK)
			err2 := json.NewEncoder(w).Encode(databases)
			if err2 != nil {
				log.Println(err2)
			}
		}
	}
}

func (e *Endpoint) readExportDatabase(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userId, _ := getUserInfo(req)
	if userId == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(map[string]string{"error": "missing user id"})
		if err != nil {
			log.Println(err)
		}
	} else {
		vars := mux.Vars(req)
		database, errs := e.serving.GetExportDatabase(vars["id"], userId)
		if len(errs) > 0 {
			status := http.StatusInternalServerError
			for _, err := range errs {
				if gorm.IsRecordNotFoundError(err) {
					status = http.StatusNotFound
				}
			}
			w.WriteHeader(status)
			err2 := json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprint(errs)})
			if err2 != nil {
				log.Println(err2)
			}
		} else {
			w.WriteHeader(http.StatusOK)
			err3 := json.NewEncoder(w).Encode(database)
			if err3 != nil {
				log.Println(err3)
			}
		}
	}
}

func (e *Endpoint) writeExportDatabase(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userId, _ := getUserInfo(req)
	if userId == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(map[string]string{"error": "missing user id"})
		if err != nil {
			log.Println(err)
		}
	} else {
		decoder := json.NewDecoder(req.Body)
		defer req.Body.Close()
		var databaseReq lib.ExportDatabaseRequest
		err := decoder.Decode(&databaseReq)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			err2 := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			if err2 != nil {
				log.Println(err2)
			}
		} else {
			validated, verrs := ValidateInputs(databaseReq)
			if validated {
				var database lib.ExportDatabase
				var errs []error
				switch req.Method {
				case http.MethodPost:
					database, errs = e.serving.CreateExportDatabase("", databaseReq, userId)
				case http.MethodPut:
					vars := mux.Vars(req)
					database, errs = e.serving.UpdateExportDatabase(vars["id"], databaseReq, userId)
				}
				if len(errs) > 0 {
					w.WriteHeader(http.StatusInternalServerError)
					err3 := json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprint(errs)})
					if err3 != nil {
						log.Println(err3)
					}
				} else {
					w.WriteHeader(http.StatusOK)
					err4 := json.NewEncoder(w).Encode(database)
					if err4 != nil {
						log.Println(err4)
					}
				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				err5 := json.NewEncoder(w).Encode(map[string]map[string][]string{"validation_errors": verrs})
				if err5 != nil {
					log.Println(err5)
				}
			}
		}
	}
}

func (e *Endpoint) deleteExportDatabase(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(req)
	userId, _ := getUserInfo(req)
	if userId == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(map[string]string{"error": "missing user id"})
		if err != nil {
			log.Println(err)
		}
	} else {
		errs := e.serving.DeleteExportDatabase(vars["id"], userId)
		if len(errs) > 0 {
			status := http.StatusInternalServerError
			for _, err := range errs {
				if gorm.IsRecordNotFoundError(err) {
					status = http.StatusNotFound
				}
			}
			w.WriteHeader(status)
			err2 := json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprint(errs)})
			if err2 != nil {
				log.Println(err2)
			}
		} else {
			w.WriteHeader(http.StatusOK)
		}
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
