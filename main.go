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
	"fmt"
	"log"
	"os"

	api_doc "github.com/SENERGY-Platform/analytics-serving/internal/api-doc"
	"github.com/SENERGY-Platform/analytics-serving/pkg/api"
	"github.com/SENERGY-Platform/analytics-serving/pkg/config"
	"github.com/SENERGY-Platform/analytics-serving/pkg/db"
	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	"github.com/SENERGY-Platform/go-service-base/srv-info-hdl"
	sb_util "github.com/SENERGY-Platform/go-service-base/util"
)

var Version = "0.0.31"

func main() {
	defer func() {
		//os.Exit(ec)
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

	db.Init(&cfg.MySQL)
	defer db.Close()
	m := db.NewMigration(db.GetDB(), cfg.MigrationInfo)
	m.Migrate()
	err = m.TmpMigrate()
	if err != nil {
		log.Println(err)
		return
	}
	api_doc.PublishAsyncapiDoc()
	api.StartServer(cfg)
}
