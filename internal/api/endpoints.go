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
	"encoding/json"
	"fmt"
	"github.com/SENERGY-Platform/analytics-serving/internal/lib"
	"github.com/golang-jwt/jwt"
	"github.com/jinzhu/gorm"
	"log"
	"net/http"
	"os"
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

// postNewServingInstance godoc
// @Summary Create export
// @Description Create an export to the serving layer.
// @Tags Export
// @Accept json
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param request body lib.ServingRequest true "request data"
// @Success	200 {object} lib.Instance "export"
// @Failure	400 {object} map[string]map[string][]string "error data"
// @Failure	500
// @Router /instance [post]
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

// putNewServingInstance godoc
// @Summary Update export
// @Description Update an export.
// @Tags Export
// @Accept json
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param id path string true "export id"
// @Param request body lib.ServingRequest true "request data"
// @Success	200 {object} lib.Instance "export"
// @Failure	400 {object} map[string]map[string][]string "error data"
// @Failure	500
// @Router /instance/{id} [put]
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

// getServingInstance godoc
// @Summary Get export
// @Description Retrieve an export.
// @Tags Export
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param id path string true "export id"
// @Success	200 {object} lib.Instance "export"
// @Failure	500
// @Router /instance/{id} [get]
func (e *Endpoint) getServingInstance(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Set("Content-Type", "application/json")
	token := getToken(req)
	userId, isAdmin := getUserInfoFromToken(token)
	instance, errors := e.serving.GetInstance(vars["id"], userId, token, isAdmin)
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

// getServingInstances godoc
// @Summary Get exports
// @Description Get all exports.
// @Tags Export
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Param order query string false "order"
// @Param search query string false "search"
// @Param generated query string false "generated"
// @Param export_database_id query string false "export_database_id"
// @Param internal_only query string false "internal_only"
// @Success	200 {array} lib.Instance "exports"
// @Failure	500
// @Router /instance [get]
func (e *Endpoint) getServingInstances(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	args := req.URL.Query()
	token := getToken(req)
	userId, _ := getUserInfoFromToken(token)
	instances, total, errors := e.serving.GetInstancesForUser(userId, args, token)
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

// deleteServingInstance godoc
// @Summary Delete export
// @Description Remove an export.
// @Tags Export
// @Accept json
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param id path string true "export id"
// @Success	200
// @Success	204
// @Failure	207 {object} map[string]string ""
// @Failure	404
// @Failure	500 {object} map[string]string ""
// @Router /instance/{id} [delete]
func (e *Endpoint) deleteServingInstance(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	token := getToken(req)
	userId, _ := getUserInfoFromToken(token)
	deleted, errors := e.serving.DeleteInstanceForUser(vars["id"], userId, token)
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

// deleteServingInstances godoc
// @Summary Delete exports
// @Description Remove multiple exports.
// @Tags Export
// @Accept json
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param ids body []string true "export ids"
// @Success	200
// @Success	204
// @Success	207 {object} lib.Response ""
// @Router /instances [delete]
func (e *Endpoint) deleteServingInstances(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var requestBody []string
	err := decoder.Decode(&requestBody)
	if err != nil {
		log.Println(err)
	}
	defer req.Body.Close()
	token := getToken(req)
	userId, _ := getUserInfoFromToken(token)
	deleted, errors := e.serving.DeleteInstancesForUser(requestBody, userId, token)
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

// getServingInstancesAdmin godoc
// @Summary Get exports
// @Description Get all exports
// @Tags Export
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Success	200 {array} lib.Instance "exports"
// @Failure	500
// @Router /admin/instance [get]
func (e *Endpoint) getServingInstancesAdmin(w http.ResponseWriter, req *http.Request) {
	args := req.URL.Query()
	token := getToken(req)
	userId, admin := getUserInfoFromToken(token)
	w.Header().Set("Content-Type", "application/json")
	if admin {
		instances, _, errors := e.serving.GetInstances(userId, args, admin, token)
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

// deleteServingInstanceAdmin godoc
// @Summary Delete export
// @Description Remove an export.
// @Tags Export
// @Produce	json,plain
// @Param Authorization header string true "jwt token"
// @Param id path string true "export id"
// @Success	204
// @Success 207 {object} map[string]string ""
// @Failure	404
// @Failure	500 {object} map[string]string ""
// @Router /admin/instance/{id} [delete]
func (e *Endpoint) deleteServingInstanceAdmin(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	token := getToken(req)
	userId, admin := getUserInfoFromToken(token)
	deleted, errors := e.serving.DeleteInstanceWithPermHandling(vars["id"], userId, admin, token)
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

// getExportDatabases godoc
// @Summary Get databases
// @Description List all export databases
// @Tags Export Database
// @Accept json,plain
// @Produce	json,plain
// @Param Authorization header string true "jwt token"
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Param order query string false "order"
// @Param search query string false "search"
// @Param deployment query string false "deployment"
// @Param owner query string false "owner"
// @Success	200 {array} lib.ExportDatabase "databases"
// @Failure	400 {object} map[string]string "error message"
// @Failure	500 {object} map[string]string "error message"
// @Router /databases [get]
func (e *Endpoint) getExportDatabases(w http.ResponseWriter, req *http.Request) {
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

// getExportDatabase godoc
// @Summary Get database
// @Description Get an export database.
// @Tags Export Database
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param id path string true "database id"
// @Success	200 {object} lib.ExportDatabase "database"
// @Failure	400 {object} map[string]string "error message"
// @Failure	404 {object} map[string]string "error message"
// @Failure	500 {object} map[string]string "error message"
// @Router /databases/{id} [get]
func (e *Endpoint) getExportDatabase(w http.ResponseWriter, req *http.Request) {
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

// postExportDatabase godoc
// @Summary Create database
// @Description Create an export database.
// @Tags Export Database
// @Accept json
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param request body lib.ExportDatabaseRequest true "export database data"
// @Success	200 {object} lib.ExportDatabase ""
// @Failure	400 {object} map[string]string "error message"
// @Failure	500 {object} map[string]string "error message"
// @Router /databases [post]
func (e *Endpoint) postExportDatabase(w http.ResponseWriter, req *http.Request) {
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
				database, errs := e.serving.CreateExportDatabase("", databaseReq, userId)
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

// putExportDatabase godoc
// @Summary Update database
// @Description Update an export database.
// @Tags Export Database
// @Accept json
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param id path string true "database id"
// @Param request body lib.ExportDatabaseRequest true "export database data"
// @Success	200 {object} lib.ExportDatabase "export database"
// @Failure	400 {object} map[string]string "error message"
// @Failure	404 {object} map[string]string "error message"
// @Failure	500 {object} map[string]string "error message"
// @Router /databases/{id} [put]
func (e *Endpoint) putExportDatabase(w http.ResponseWriter, req *http.Request) {
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
				vars := mux.Vars(req)
				database, errs := e.serving.UpdateExportDatabase(vars["id"], databaseReq, userId)
				if len(errs) > 0 {
					status := http.StatusInternalServerError
					for _, e := range errs {
						if gorm.IsRecordNotFoundError(e) {
							status = http.StatusNotFound
						}
					}
					w.WriteHeader(status)
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

// deleteExportDatabase godoc
// @Summary Delete database
// @Description Remove an export database.
// @Tags Export Database
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param id path string true "database id"
// @Success	200
// @Failure	400 {object} map[string]string "error message"
// @Failure	404 {object} map[string]string "error message"
// @Failure	500 {object} map[string]string "error message"
// @Router /databases/{id} [delete]
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

// getSwaggerDoc godoc
// @Summary Get swagger
// @Tags Documentation
// @Produce	json
// @Success	200 {object} map[string]any "swagger file"
// @Failure	500 {object} map[string]string "error message"
// @Router /doc [get]
func (e *Endpoint) getSwaggerDoc(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	file, err := os.Open("docs/swagger.json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		if err != nil {
			log.Println(err)
		}
		return
	}
	defer file.Close()
	_, err = file.WriteTo(w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		if err != nil {
			log.Println(err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

// getAsyncapiDoc godoc
// @Summary Get asyncapi
// @Tags Documentation
// @Produce	json
// @Success	200 {object} map[string]any "asyncapi file"
// @Failure	500 {object} map[string]string "error message"
// @Router /async-doc [get]
func (e *Endpoint) getAsyncapiDoc(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	file, err := os.Open("docs/asyncapi.json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		if err != nil {
			log.Println(err)
		}
		return
	}
	defer file.Close()
	_, err = file.WriteTo(w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		if err != nil {
			log.Println(err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

func getToken(req *http.Request) string {
	return req.Header.Get("Authorization")
}

func getUserInfoFromToken(token string) (userId string, admin bool) {
	if len(token) > 0 {
		_, claims := parseJWTToken(token[7:])
		userId = claims.Sub
		admin = claims.IsAdmin()
		if userId == "" {
			userId = "dummy"
			admin = false
		}
	}
	return
}

func getUserInfo(req *http.Request) (userId string, admin bool) {
	return getUserInfoFromToken(getToken(req))
}

func parseJWTToken(encodedToken string) (token *jwt.Token, claims lib.Claims) {
	token, _ = jwt.ParseWithClaims(encodedToken, &claims, nil)
	return
}
