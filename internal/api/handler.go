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
	rancher_api "analytics-serving/internal/rancher-api"
	rancher2_api "analytics-serving/internal/rancher2-api"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func CreateServer() {
	var driver lib.Driver
	switch selectedDriver := lib.GetEnv("DRIVER", "rancher"); selectedDriver {
	case "rancher":
		fmt.Println("using rancher driver")
		driver = rancher_api.NewRancher(
			lib.GetEnv("RANCHER_ENDPOINT", ""),
			lib.GetEnv("RANCHER_ACCESS_KEY", ""),
			lib.GetEnv("RANCHER_SECRET_KEY", ""),
			lib.GetEnv("RANCHER_STACK_ID", ""),
		)
	case "rancher2":
		fmt.Println("using rancher2 driver")
		driver = rancher2_api.NewRancher2(
			lib.GetEnv("RANCHER2_ENDPOINT", ""),
			lib.GetEnv("RANCHER2_ACCESS_KEY", ""),
			lib.GetEnv("RANCHER2_SECRET_KEY", ""),
			lib.GetEnv("RANCHER2_NAMESPACE_ID", ""),
			lib.GetEnv("RANCHER2_PROJECT_ID", ""),
		)
	default:
		fmt.Println("No driver selected")
	}

	port := lib.GetEnv("API_PORT", "8000")
	fmt.Print("Starting Server at port " + port + "\n")
	router := mux.NewRouter()

	e := NewEndpoint(driver)
	router.HandleFunc("/", e.getRootEndpoint).Methods("GET")
	router.HandleFunc("/instance", e.putNewServingInstance).Methods("PUT")
	router.HandleFunc("/instance", e.getServingInstances).Methods("GET")
	router.HandleFunc("/instance/{id}", e.getServingInstance).Methods("GET")
	router.HandleFunc("/instance/{id}", e.deleteServingInstance).Methods("DELETE")
	c := cors.New(
		cors.Options{
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "PUT", "POST", "DELETE", "OPTIONS"},
		})
	handler := c.Handler(router)
	logger := lib.NewLogger(handler, lib.GetEnv("LOG_LEVEL", "CALL"))
	defer logger.CloseLogFile()
	log.Fatal(http.ListenAndServe(":"+port, logger))
}
