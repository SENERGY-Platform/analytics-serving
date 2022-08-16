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
	ew_api "analytics-serving/internal/ew-api"
	import_deploy_api "analytics-serving/internal/import-deploy-api"
	"analytics-serving/internal/lib"
	permission_api "analytics-serving/internal/permission-api"
	pipeline_api "analytics-serving/internal/pipeline-api"
	rancher_api "analytics-serving/internal/rancher-api"
	rancher2_api "analytics-serving/internal/rancher2-api"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/segmentio/kafka-go"
	"log"
	"net"
	"net/http"
	"strconv"
)

func CreateServer() {
	var err error
	var driver lib.Driver
	selectedDriver := lib.GetEnv("DRIVER", "rancher")
	switch selectedDriver {
	case "rancher":
		log.Println("using rancher driver")
		driver = rancher_api.NewRancher(
			lib.GetEnv("RANCHER_ENDPOINT", ""),
			lib.GetEnv("RANCHER_ACCESS_KEY", ""),
			lib.GetEnv("RANCHER_SECRET_KEY", ""),
			lib.GetEnv("RANCHER_STACK_ID", ""),
		)
	case "rancher2":
		log.Println("using rancher2 driver")
		driver = rancher2_api.NewRancher2(
			lib.GetEnv("RANCHER2_ENDPOINT", ""),
			lib.GetEnv("RANCHER2_ACCESS_KEY", ""),
			lib.GetEnv("RANCHER2_SECRET_KEY", ""),
			lib.GetEnv("RANCHER2_NAMESPACE_ID", ""),
			lib.GetEnv("RANCHER2_PROJECT_ID", ""),
		)
	case "ew":
		log.Println("using export-worker driver")
		addr := lib.GetEnv("KAFKA_BOOTSTRAP", "")
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
	default:
		log.Println("No driver selected")
	}
	permission := permission_api.NewPermissionApi(lib.GetEnv("PERMISSION_API_ENDPOINT", ""))
	pipeline := pipeline_api.NewPipelineApi(lib.GetEnv("PIPELINE_API_ENDPOINT", ""))
	imp := import_deploy_api.NewImportDeployApi(lib.GetEnv("IMPORT_DEPLOY_API_ENDPOINT", ""))
	serving := lib.NewServing(driver, permission, pipeline, imp, lib.GetEnv("EXPORT_DATABASE_ID_PREFIX", ""))
	if drvr, ok := driver.(lib.ExportWorkerKafkaApi); ok {
		err = drvr.InitFilterTopics(serving)
		if err != nil {
			log.Fatal(err)
			return
		}
	}
	port := lib.GetEnv("API_PORT", "8000")
	log.Print("Starting Server at port " + port + "\n")
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
	router.HandleFunc("/databases", e.readExportDatabases).Methods("GET")
	router.HandleFunc("/databases", e.writeExportDatabase).Methods("POST")
	router.HandleFunc("/databases/{id}", e.readExportDatabase).Methods("GET")
	router.HandleFunc("/databases/{id}", e.writeExportDatabase).Methods("PUT")
	router.HandleFunc("/databases/{id}", e.deleteExportDatabase).Methods("DELETE")
	c := cors.New(
		cors.Options{
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "PUT", "POST", "DELETE", "OPTIONS"},
		})
	handler := c.Handler(router)
	logger := lib.NewLogger(handler, lib.GetEnv("LOG_LEVEL", "CALL"))
	defer logger.CloseLogFile()
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Fatal(http.ListenAndServe(lib.GetEnv("SERVERNAME", "")+":"+port, logger))
}
