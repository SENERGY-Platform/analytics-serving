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

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/SENERGY-Platform/analytics-serving/pkg/api"
	api_doc "github.com/SENERGY-Platform/analytics-serving/pkg/api-doc"
	ew_api "github.com/SENERGY-Platform/analytics-serving/pkg/apis/ew-api"
	import_deploy_api "github.com/SENERGY-Platform/analytics-serving/pkg/apis/import-deploy-api"
	pipeline_api "github.com/SENERGY-Platform/analytics-serving/pkg/apis/pipeline-api"
	"github.com/SENERGY-Platform/analytics-serving/pkg/config"
	"github.com/SENERGY-Platform/analytics-serving/pkg/db"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service"
	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	"github.com/SENERGY-Platform/go-service-base/srv-info-hdl"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	sb_util "github.com/SENERGY-Platform/go-service-base/util"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/segmentio/kafka-go"
)

var Version = "0.0.33"

func main() {
	ec := 0
	defer func() {
		os.Exit(ec)
	}()

	srvInfoHdl := srv_info_hdl.New("serving-service", Version)

	config.ParseFlags()

	cfg, err := config.New(config.ConfPath)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		ec = 1
		return
	}

	util.InitStructLogger(cfg.Logger.Level)

	util.Logger.Info(srvInfoHdl.Name(), "version", srvInfoHdl.Version())
	util.Logger.Info("config: " + sb_util.ToJsonStr(cfg))

	err = db.Init(&cfg.MySQL)
	if err != nil {
		util.Logger.Error("failed to connect to database", "error", err)
		ec = 1
		return
	}
	m := db.NewMigration(db.GetDB(), cfg.MigrationInfo)
	m.Migrate()
	err = m.TmpMigrate()
	if err != nil {
		util.Logger.Error("failed to migrate", "error", err)
		ec = 1
		return
	}

	api_doc.PublishAsyncapiDoc(cfg.ApiDocsProviderBaseUrl)

	ctx, cf := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}

	var driver service.Driver
	selectedDriver := cfg.Driver
	switch selectedDriver {
	default:
		util.Logger.Info("using export-worker driver")
		addr := cfg.Kafka.Bootstrap
		kafkaProducer := kafka.Writer{
			Addr:        kafka.TCP(addr),
			MaxAttempts: 5,
			Async:       false,
			BatchSize:   1,
			Balancer:    &kafka.Hash{},
		}
		go func() {
			wg.Add(1)
			<-ctx.Done()
			_ = kafkaProducer.Close()
			util.Logger.Info("closed kafka producer connection")
			wg.Done()
		}()

		var kafkaConn *kafka.Conn
		kafkaConn, err = kafka.Dial("tcp", addr)
		if err != nil {
			return
		}
		go func() {
			wg.Add(1)
			<-ctx.Done()
			_ = kafkaConn.Close()
			util.Logger.Info("closed kafka connection")
			wg.Done()
		}()

		var controller kafka.Broker
		controller, err = kafkaConn.Controller()
		if err != nil {
			return
		}
		var kafkaControllerConn *kafka.Conn
		kafkaControllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
		if err != nil {
			return
		}
		go func() {
			wg.Add(1)
			<-ctx.Done()
			_ = kafkaControllerConn.Close()
			util.Logger.Info("closed kafka controller connection")
			wg.Done()
		}()
		driver = ew_api.NewExportWorker(cfg.Kafka, &kafkaProducer, kafkaConn, kafkaControllerConn)
	}

	go func() {
		util.Wait(ctx, util.Logger, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		cf()
	}()

	var pipeline service.PipelineApiService
	pipeline = pipeline_api.NewPipelineApi(cfg.PipelineApiUrl)

	var imp service.ImportDeployService
	imp = import_deploy_api.NewImportDeployApi(cfg.ImportDeployApiUrl)

	var influx service.Influx
	influx = service.NewInflux(cfg.InfluxConfig, ctx, wg)

	var permV2 permV2Client.Client
	if cfg.PermissionV2Url == "mock" {
		util.Logger.Debug("using mock permissions")
		permV2, err = permV2Client.NewTestClient(context.Background())
	} else {
		permV2 = permV2Client.New(cfg.PermissionV2Url)
	}

	httpHandler, err := api.CreateServer(cfg, &driver, &pipeline, &imp, &permV2, &influx)
	if err != nil {
		util.Logger.Error("error creating http engine", "error", err)
		ec = 1
		return
	}

	bindAddress := ":" + strconv.FormatInt(int64(cfg.ServerPort), 10)

	if cfg.Debug {
		bindAddress = "127.0.0.1:" + strconv.FormatInt(int64(cfg.ServerPort), 10)
	}

	httpServer := &http.Server{
		Addr:    bindAddress,
		Handler: httpHandler}

	wg.Add(1)

	go func() {
		defer wg.Done()
		util.Logger.Info("starting http server")
		if err = httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			util.Logger.Error("starting server failed", attributes.ErrorKey, err)
			ec = 1
		}
		cf()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		util.Logger.Info("stopping http server")
		ctxWt, cf2 := context.WithTimeout(context.Background(), time.Second*5)
		defer cf2()
		if err = httpServer.Shutdown(ctxWt); err != nil {
			util.Logger.Error("stopping server failed", attributes.ErrorKey, err)
			ec = 1
		} else {
			util.Logger.Info("http server stopped")
		}

		util.Logger.Info("closing db connection")
		if err = db.Close(); err != nil {
			util.Logger.Error("failed to close db connection", "error", err)
			ec = 1
		} else {
			util.Logger.Info("db connection closed")
		}
	}()

	wg.Wait()
}
