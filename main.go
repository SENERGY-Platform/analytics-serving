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
	"net/http"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/SENERGY-Platform/analytics-serving/pkg/api"
	api_doc "github.com/SENERGY-Platform/analytics-serving/pkg/api-doc"
	"github.com/SENERGY-Platform/analytics-serving/pkg/config"
	"github.com/SENERGY-Platform/analytics-serving/pkg/db"
	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	"github.com/SENERGY-Platform/go-service-base/srv-info-hdl"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	sb_util "github.com/SENERGY-Platform/go-service-base/util"
)

var Version = "0.0.31"

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
		_ = 1
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

	go func() {
		util.Wait(ctx, util.Logger, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		cf()
	}()

	httpHandler, err := api.CreateServer(cfg, ctx, wg)
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
