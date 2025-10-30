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
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	ew_api "github.com/SENERGY-Platform/analytics-serving/internal/ew-api"
	import_deploy_api "github.com/SENERGY-Platform/analytics-serving/internal/import-deploy-api"
	permission_api "github.com/SENERGY-Platform/analytics-serving/internal/permission-api"
	pipeline_api "github.com/SENERGY-Platform/analytics-serving/internal/pipeline-api"
	"github.com/SENERGY-Platform/analytics-serving/pkg/config"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service"
	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/segmentio/kafka-go"
)

func StartServer(cfg *config.Config) {
	var err error
	var driver service.Driver
	selectedDriver := cfg.Driver
	switch selectedDriver {
	default:
		log.Println("using export-worker driver")
		addr := cfg.KafkaBootstrap
		kafkaProducer := kafka.Writer{
			Addr:        kafka.TCP(addr),
			MaxAttempts: 5,
			Async:       false,
			BatchSize:   1,
			Balancer:    &kafka.Hash{},
		}
		defer func(producer *kafka.Writer) {
			_ = producer.Close()
		}(&kafkaProducer)
		var kafkaConn *kafka.Conn
		kafkaConn, err = kafka.Dial("tcp", addr)
		if err != nil {
			log.Fatal(err)
			return
		}
		defer func(conn *kafka.Conn) {
			_ = conn.Close()
		}(kafkaConn)
		var controller kafka.Broker
		controller, err = kafkaConn.Controller()
		if err != nil {
			log.Fatal(err)
			return
		}
		var kafkaControllerConn *kafka.Conn
		kafkaControllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
		if err != nil {
			log.Fatal(err)
			return
		}
		defer func(controllerConn *kafka.Conn) {
			_ = controllerConn.Close()
		}(kafkaControllerConn)
		driver = ew_api.NewExportWorker(&kafkaProducer, kafkaConn, kafkaControllerConn)
	}
	permission := permission_api.NewPermissionApi(cfg.PermissionV2Url)
	pipeline := pipeline_api.NewPipelineApi(cfg.PipelineApiUrl)
	imp := import_deploy_api.NewImportDeployApi(cfg.ImportDeployApiUrl)
	var permV2 permV2Client.Client
	if cfg.PermissionV2Url == "mock" {
		util.Logger.Debug("using mock permissions")
		permV2, err = permV2Client.NewTestClient(context.Background())
	} else {
		permV2 = permV2Client.New(cfg.PermissionV2Url)
	}
	influx := service.NewInflux(cfg.InfluxConfig)
	server, _, err := CreateServerFromDependencies(driver, cfg, influx, permission, permV2, pipeline, imp)
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Println("Starting Server at " + server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		debug.PrintStack()
		log.Fatal("FATAL:", err)
	}
}

// CreateServerFromDependencies godoc
// @title Analytics Serving Service API
// @version 0.0.0
// @license.name Apache-2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath /
func CreateServerFromDependencies(driver service.Driver, cfg *config.Config, influx service.Influx, permission service.PermissionApiService, permV2 permV2Client.Client, pipeline service.PipelineApiService, imp service.ImportDeployService) (*http.Server, *service.Serving, error) {
	var err error
	cleanupWait, err := time.ParseDuration(cfg.CleanupConfig.WaitDuration)
	if err != nil {
		return nil, nil, err
	}
	var serving *service.Serving
	serving, err = service.NewServing(driver,
		influx,
		permission,
		pipeline,
		imp,
		cfg.ExportDatabaseIdPrefix,
		permV2,
		cfg.CleanupConfig.Cron,
		cleanupWait)
	if err != nil {
		return nil, nil, err
	}
	if drvr, ok := driver.(service.ExportWorkerKafkaApi); ok {
		err = drvr.InitFilterTopics(serving)
		if err != nil {
			return nil, nil, err
		}
	}
	router := mux.NewRouter()
	e := NewEndpoint(serving)
	router.HandleFunc("/", e.getRootEndpoint).Methods("GET")
	router.HandleFunc("/instance", e.postNewServingInstance).Methods("POST")
	router.HandleFunc("/instance", e.getServingInstances).Methods("GET")
	router.HandleFunc("/instance/{id}", e.putNewServingInstance).Methods("PUT")
	router.HandleFunc("/instance/{id}", e.getServingInstance).Methods("GET")
	router.HandleFunc("/instance/{id}", e.deleteServingInstance).Methods("DELETE")
	router.HandleFunc("/instances", e.deleteServingInstances).Methods("DELETE")
	router.HandleFunc("/admin/instance", e.getServingInstancesAdmin).Methods("GET")
	router.HandleFunc("/admin/instance/{id}", e.deleteServingInstanceAdmin).Methods("DELETE")
	router.HandleFunc("/databases", e.getExportDatabases).Methods("GET")
	router.HandleFunc("/databases", e.postExportDatabase).Methods("POST")
	router.HandleFunc("/databases/{id}", e.getExportDatabase).Methods("GET")
	router.HandleFunc("/databases/{id}", e.putExportDatabase).Methods("PUT")
	router.HandleFunc("/databases/{id}", e.deleteExportDatabase).Methods("DELETE")
	router.HandleFunc("/doc", e.getSwaggerDoc).Methods("GET")
	router.HandleFunc("/async-doc", e.getAsyncapiDoc).Methods("GET")
	c := cors.New(
		cors.Options{
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "PUT", "POST", "DELETE", "OPTIONS"},
		})
	handler := c.Handler(router)
	port := cfg.ServerPort
	return &http.Server{Addr: ":" + strconv.Itoa(port), Handler: handler}, serving, nil
}
