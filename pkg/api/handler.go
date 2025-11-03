/*
 * Copyright 2025 InfAI (CC SES)
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
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/SENERGY-Platform/analytics-serving/lib"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service"
	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

// postNewServingInstance godoc
// @Summary Create export
// @Description Create an export to the serving layer.
// @Tags Export
// @Accept json
// @Produce	json
// @Param Authorization header string true "jwt token"
// @Param request body lib.ServingRequest true "request data"
// @Success	201 {object} lib.Instance "export"
// @Failure	400 {object} map[string]map[string][]string "error data"
// @Failure	500
// @Router /instance [post]
func postNewServingInstance(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodPost, "/instance", func(c *gin.Context) {
		var request lib.ServingRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			util.Logger.Error(MessageParseError, "error", err)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}

		validated, errs := ValidateInputs(request)

		if !validated {
			c.JSON(http.StatusBadRequest, map[string]map[string][]string{"validationErrors": errs})
			return
		}
		instance, err := serv.CreateInstance(request, c.GetString(UserIdKey), c.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("could not create serving instance", "error", err)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		c.JSON(http.StatusCreated, instance)
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
func putNewServingInstance(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodPut, "/instance/:id", func(c *gin.Context) {
		var request lib.ServingRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			util.Logger.Error(MessageParseError, "error", err)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}

		validated, valErrs := ValidateInputs(request)

		if !validated {
			c.JSON(http.StatusBadRequest, map[string]map[string][]string{"validationErrors": valErrs})
			return
		}
		instance, errs := serv.UpdateInstance(c.Param("id"), c.GetString(UserIdKey), request, c.GetHeader("Authorization"))
		if len(errs) > 0 {
			util.Logger.Error("could not update serving instance", "error", errs)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		c.JSON(http.StatusOK, instance)
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
func getServingInstance(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/instance/:id", func(c *gin.Context) {
		admin, err := isAdmin(c)
		if err != nil {
			util.Logger.Error("could not check admin status", "error", err)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		instance, errs := serv.GetInstance(c.Param("id"), c.GetString(UserIdKey), c.GetHeader("Authorization"), admin)
		if len(errs) > 0 {
			util.Logger.Error("could not get serving instance", "error", errs)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		c.JSON(http.StatusOK, instance)
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
func getServingInstances(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/instance", func(c *gin.Context) {
		args := c.Request.URL.Query()
		instances, total, errs := serv.GetInstancesForUser(c.GetString(UserIdKey), args, c.GetHeader("Authorization"))
		if len(errs) > 0 {
			util.Logger.Error("could not get serving instances", "error", errs)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		c.JSON(http.StatusOK, lib.InstancesResponse{
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
func deleteServingInstance(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodDelete, "/instance/:id", func(c *gin.Context) {
		deleted, errs := serv.DeleteInstanceForUser(c.Param("id"), c.GetString(UserIdKey), c.GetHeader("Authorization"))
		if len(errs) > 0 {
			util.Logger.Error("could not delete serving instance", "error", errs)
		}
		if len(errs) == 0 && deleted == true {
			c.Status(http.StatusNoContent)
		} else if len(errs) == 0 && deleted == false {
			c.Status(http.StatusNotFound)
		} else if len(errs) > 0 && deleted == true {
			c.Status(http.StatusMultiStatus)
		} else {
			c.JSON(http.StatusInternalServerError, map[string]string{"deleted": fmt.Sprint(deleted), "error": fmt.Sprint(errs)})
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
func deleteServingInstances(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodDelete, "/instances", func(c *gin.Context) {
		var request []string
		if err := c.ShouldBindJSON(&request); err != nil {
			util.Logger.Error(MessageParseError, "error", err)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		deleted, errs := serv.DeleteInstancesForUser(request, c.GetString(UserIdKey), c.GetHeader("Authorization"))

		if len(errs) > 0 {
			util.Logger.Error("could not delete serving instances", "error", errs)
		}

		if len(errs) > 0 && len(deleted) < 1 {
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		} else if len(errs) > 0 && len(deleted) > 0 {
			c.JSON(http.StatusMultiStatus, lib.Response{
				Message: "Could not delete all exports. Exports deleted: " + strings.Join(deleted, ","),
			})
		} else {
			c.Status(http.StatusNoContent)
		}
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
func getServingInstancesAdmin(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/admin/instance", func(c *gin.Context) {
		args := c.Request.URL.Query()
		instances, _, errs := serv.GetInstances(c.GetString(UserIdKey), args, true, c.GetHeader("Authorization"))
		if len(errs) > 0 {
			util.Logger.Error("could not get serving instances for admin", "error", errs)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		c.JSON(http.StatusOK, instances)
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
func deleteServingInstanceAdmin(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodDelete, "/admin/instance/:id", func(c *gin.Context) {
		deleted, errs := serv.DeleteInstanceWithPermHandling(c.Param("id"), c.GetString(UserIdKey), true, c.GetHeader("Authorization"))
		if len(errs) > 0 {
			util.Logger.Error("could not delete serving instance for admin", "error", errs)
		}
		if len(errs) == 0 && deleted == true {
			c.Status(http.StatusNoContent)
		} else if len(errs) == 0 && deleted == false {
			c.Status(http.StatusNotFound)
		} else if len(errs) > 0 && deleted == true {
			c.Status(http.StatusMultiStatus)
		} else {
			c.JSON(http.StatusInternalServerError, map[string]string{"deleted": fmt.Sprint(deleted), "error": fmt.Sprint(errs)})
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
func getExportDatabases(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/databases", func(c *gin.Context) {
		args := c.Request.URL.Query()
		databases, errs := serv.GetExportDatabases(c.GetString(UserIdKey), args)
		if len(errs) > 0 {
			util.Logger.Error("could not get export databases", "error", errs)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		c.JSON(http.StatusOK, databases)
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
func getExportDatabase(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/databases/:id", func(c *gin.Context) {
		database, errs := serv.GetExportDatabase(c.Param("id"), c.GetString(UserIdKey))
		if len(errs) > 0 {
			for _, err := range errs {
				if gorm.IsRecordNotFoundError(err) {
					c.Status(http.StatusNotFound)
					return
				}
			}
			util.Logger.Error("could not get export database", "error", errs)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		c.JSON(http.StatusOK, database)
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
func postExportDatabase(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodPost, "/databases", func(c *gin.Context) {
		var request lib.ExportDatabaseRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			util.Logger.Error(MessageParseError, "error", err)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}

		validated, valErrs := ValidateInputs(request)

		if !validated {
			c.JSON(http.StatusBadRequest, map[string]map[string][]string{"validationErrors": valErrs})
			return
		}

		database, errs := serv.CreateExportDatabase("", request, c.GetString(UserIdKey))
		if len(errs) > 0 {
			util.Logger.Error("could not create export database", "error", errs)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		c.JSON(http.StatusOK, database)
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
func putExportDatabase(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodPut, "/databases/:id", func(c *gin.Context) {
		var request lib.ExportDatabaseRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			util.Logger.Error(MessageParseError, "error", err)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}

		validated, valErrs := ValidateInputs(request)

		if !validated {
			c.JSON(http.StatusBadRequest, map[string]map[string][]string{"validationErrors": valErrs})
			return
		}

		database, errs := serv.UpdateExportDatabase(c.Param("id"), request, c.GetString(UserIdKey))
		if len(errs) > 0 {
			for _, err := range errs {
				if gorm.IsRecordNotFoundError(err) {
					c.Status(http.StatusNotFound)
					return
				}
			}
			util.Logger.Error("could not update export database", "error", errs)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		c.JSON(http.StatusOK, database)
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
func deleteExportDatabase(serv *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodDelete, "/databases/:id", func(c *gin.Context) {

		errs := serv.DeleteExportDatabase(c.Param("id"), c.GetString(UserIdKey))
		if len(errs) > 0 {
			for _, err := range errs {
				if gorm.IsRecordNotFoundError(err) {
					c.Status(http.StatusNotFound)
					return
				}
			}
			util.Logger.Error("could not delete export database", "error", errs)
			_ = c.Error(errors.New(MessageSomethingWrong))
			return
		}
		c.Status(http.StatusOK)
	}
}

func getHealthCheckH(_ *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodGet, HealthCheckPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	}
}

func getSwaggerDocH(_ *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/doc", func(gc *gin.Context) {
		if _, err := os.Stat("docs/swagger.json"); err != nil {
			_ = gc.Error(err)
			return
		}
		gc.Header("Content-Type", gin.MIMEJSON)
		gc.File("docs/swagger.json")
	}
}

func getAsyncapiDocH(_ *service.Serving) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/async-doc", func(gc *gin.Context) {
		if _, err := os.Stat("docs/asyncapi.json"); err != nil {
			_ = gc.Error(err)
			return
		}
		gc.Header("Content-Type", gin.MIMEJSON)
		gc.File("docs/asyncapi.json")
	}
}
