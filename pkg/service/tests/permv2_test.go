/*
 * Copyright 2024 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/SENERGY-Platform/analytics-serving/lib"
	"github.com/SENERGY-Platform/analytics-serving/pkg/api"
	"github.com/SENERGY-Platform/analytics-serving/pkg/config"
	"github.com/SENERGY-Platform/analytics-serving/pkg/db"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service/tests/docker"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service/tests/mocks"
	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/SENERGY-Platform/permissions-v2/pkg/model"
)

func TestPermissionsV2Handling(t *testing.T) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, dbIp, _, err := docker.MySqlWithNetwork(ctx, wg, "exports")
	if err != nil {
		t.Error(err)
		return
	}

	serverPortInt, err := docker.GetFreePort()
	if err != nil {
		t.Error(err)
		return
	}
	serverPort := strconv.Itoa(serverPortInt)

	t.Setenv("MYSQL_USER", "usr")
	t.Setenv("MYSQL_PW", "pw")
	t.Setenv("MYSQL_HOST", dbIp)
	t.Setenv("MYSQL_DB", "exports")

	t.Setenv("CLEANUP_WAIT_DURATION", "5s")
	t.Setenv("CLEANUP_CRON", "0 3 * * *")
	t.Setenv("SERVER_PORT", serverPort)
	t.Setenv("EXPORT_DATABASE_ID_PREFIX", "")

	//ignored
	/*
		t.Setenv("SERVERNAME", "")
		t.Setenv("PERMISSION_API_ENDPOINT", "")
		t.Setenv("PIPELINE_API_ENDPOINT", "")
		t.Setenv("IMPORT_DEPLOY_API_ENDPOINT", "")
		t.Setenv("PERMISSION_V2_URL", "")
		t.Setenv("DRIVER", "ew")
		t.Setenv("KAFKA_BOOTSTRAP", "")
		t.Setenv("INFLUX_DB_PROTO", "")
		t.Setenv("INFLUX_DB_HOST", "")
		t.Setenv("INFLUX_DB_PORT", "")
		t.Setenv("INFLUX_DB_USERNAME", "")
		t.Setenv("INFLUX_DB_PASSWORD", "")
	*/

	config.ParseFlags()

	cfg, err := config.New(config.ConfPath)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		_ = 1
		return
	}

	err = db.Init(&cfg.MySQL)
	if err != nil {
		return
	}
	defer func() {
		err = db.Close()
		if err != nil {

		}
	}()
	m := db.NewMigration(db.GetDB(), cfg.MigrationInfo)
	m.Migrate()
	err = m.TmpMigrate()
	if err != nil {
		t.Error(err)
		return
	}

	var permV2 client.Client
	permV2, err = client.NewTestClient(ctx)
	if err != nil {
		t.Error(err)
		return
	}

	var driver service.Driver
	var pipeline service.PipelineApiService
	var imp service.ImportDeployService
	var influx service.Influx
	driver = mocks.Driver{}
	pipeline = mocks.Pipeline{}
	imp = mocks.Imports{}
	influx = mocks.Influx{}

	httpHandler, err := api.CreateServer(cfg, &driver, &pipeline, &imp, &permV2, &influx)
	if err != nil {
		util.Logger.Error("error creating http engine", "error", err)
		return
	}

	cleanupWait, err := time.ParseDuration(cfg.CleanupConfig.WaitDuration)
	if err != nil {
		t.Error(err)
		return
	}
	serving, err := service.NewServing(driver,
		influx,
		pipeline,
		imp,
		cfg.ExportDatabaseIdPrefix,
		permV2,
		cfg.CleanupConfig.Cron,
		cleanupWait,
	)
	if err != nil {
		t.Error(err)
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
		}
	}()

	time.Sleep(1 * time.Second)

	var exportDatabasePrivate lib.ExportDatabase
	t.Run("create export db 1", func(t *testing.T) {
		temp, err := json.Marshal(lib.ExportDatabaseRequest{
			Name:          "testdb",
			Description:   "testdb",
			Type:          "?",
			Deployment:    "?",
			Url:           "?",
			EwFilterTopic: "?",
			Public:        false,
		})
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:"+serverPort+"/databases", bytes.NewReader(temp))
		if err != nil {
			t.Error(err)
			return
		}
		exportDatabasePrivate, err, _ = doReq[lib.ExportDatabase](TestToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		if exportDatabasePrivate.Name != "testdb" {
			t.Error(exportDatabasePrivate.Name)
		}
	})

	var exportDatabasePublic lib.ExportDatabase
	t.Run("create public export db", func(t *testing.T) {
		temp, err := json.Marshal(lib.ExportDatabaseRequest{
			Name:          "public",
			Description:   "public",
			Type:          "?",
			Deployment:    "?",
			Url:           "?",
			EwFilterTopic: "?",
			Public:        true,
		})
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:"+serverPort+"/databases", bytes.NewReader(temp))
		if err != nil {
			t.Error(err)
			return
		}
		exportDatabasePublic, err, _ = doReq[lib.ExportDatabase](SecondOwnerToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		if exportDatabasePublic.Name != "public" {
			t.Error(exportDatabasePrivate.Name)
		}
	})

	var exportInstance1 lib.Instance
	t.Run("create export instance 1", func(t *testing.T) {
		temp, err := json.Marshal(lib.ServingRequest{
			FilterType:       "deviceId",
			Name:             "instance1",
			Filter:           "device1",
			EntityName:       "device1",
			ServiceName:      "service1",
			Description:      "foo",
			Topic:            "?",
			TimePath:         "?",
			TimePrecision:    "?",
			Generated:        false,
			Offset:           "?",
			ForceUpdate:      false,
			Values:           nil,
			ExportDatabaseID: exportDatabasePublic.ID,
			TimestampFormat:  "?",
		})
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:"+serverPort+"/instance", bytes.NewReader(temp))
		if err != nil {
			t.Error(err)
			return
		}
		exportInstance1, err, _ = doReq[lib.Instance](TestToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		if exportInstance1.Name != "instance1" {
			t.Error(exportDatabasePrivate.Name)
		}
		ids, err, _ := permV2.ListAccessibleResourceIds(TestToken, service.ExportInstancePermissionsTopic, client.ListOptions{}, client.Administrate)
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(ids, []string{exportInstance1.ID.String()}) {
			t.Error(ids)
			return
		}
	})

	t.Run("create invalid export instance 2", func(t *testing.T) {
		temp, err := json.Marshal(lib.ServingRequest{
			FilterType:       "deviceId",
			Name:             "instance2",
			Filter:           "device1",
			EntityName:       "device1",
			ServiceName:      "service1",
			Description:      "foo",
			Topic:            "?",
			TimePath:         "?",
			TimePrecision:    "?",
			Generated:        false,
			Offset:           "?",
			ForceUpdate:      false,
			Values:           nil,
			ExportDatabaseID: exportDatabasePrivate.ID,
			TimestampFormat:  "?",
		})
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:"+serverPort+"/instance", bytes.NewReader(temp))
		if err != nil {
			t.Error(err)
			return
		}
		_, err, _ = doReq[lib.Instance](SecondOwnerToken, req)
		if err == nil {
			t.Error("expected error")
			return
		}
		ids, err, _ := permV2.ListAccessibleResourceIds(SecondOwnerToken, service.ExportInstancePermissionsTopic, client.ListOptions{}, client.Administrate)
		if err != nil {
			t.Error(err)
			return
		}
		if len(ids) > 0 {
			t.Error("expected no accessible resource ids")
			return
		}
	})

	var exportInstance2 lib.Instance
	t.Run("create export instance 2", func(t *testing.T) {
		temp, err := json.Marshal(lib.ServingRequest{
			FilterType:       "deviceId",
			Name:             "instance2",
			Filter:           "device1",
			EntityName:       "device1",
			ServiceName:      "service1",
			Description:      "foo",
			Topic:            "?",
			TimePath:         "?",
			TimePrecision:    "?",
			Generated:        false,
			Offset:           "?",
			ForceUpdate:      false,
			Values:           nil,
			ExportDatabaseID: exportDatabasePublic.ID,
			TimestampFormat:  "?",
		})
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:"+serverPort+"/instance", bytes.NewReader(temp))
		if err != nil {
			t.Error(err)
			return
		}
		exportInstance2, err, _ = doReq[lib.Instance](SecondOwnerToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		ids, err, _ := permV2.ListAccessibleResourceIds(SecondOwnerToken, service.ExportInstancePermissionsTopic, client.ListOptions{}, client.Administrate)
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(ids, []string{exportInstance2.ID.String()}) {
			t.Error(ids)
			return
		}
	})

	t.Run("create and delete a instance", func(t *testing.T) {
		var exportInstance lib.Instance
		temp, err := json.Marshal(lib.ServingRequest{
			FilterType:       "deviceId",
			Name:             "instanceDelete",
			Filter:           "device1",
			EntityName:       "device1",
			ServiceName:      "service1",
			Description:      "foo",
			Topic:            "?",
			TimePath:         "?",
			TimePrecision:    "?",
			Generated:        false,
			Offset:           "?",
			ForceUpdate:      false,
			Values:           nil,
			ExportDatabaseID: exportDatabasePublic.ID,
			TimestampFormat:  "?",
		})
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:"+serverPort+"/instance", bytes.NewReader(temp))
		if err != nil {
			t.Error(err)
			return
		}
		exportInstance, err, _ = doReq[lib.Instance](SecondOwnerToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		ids, err, _ := permV2.ListAccessibleResourceIds(SecondOwnerToken, service.ExportInstancePermissionsTopic, client.ListOptions{}, client.Administrate)
		if err != nil {
			t.Error(err)
			return
		}
		sort.Strings(ids)
		expected := []string{exportInstance2.ID.String(), exportInstance.ID.String()}
		sort.Strings(expected)
		if !reflect.DeepEqual(ids, expected) {
			t.Error(ids)
			return
		}

		req, err = http.NewRequest(http.MethodDelete, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance.ID.String()), nil)
		if err != nil {
			t.Error(err)
			return
		}
		err, _ = doVoid(SecondOwnerToken, req)
		if err != nil {
			t.Error(err)
			return
		}

		ids, err, _ = permV2.ListAccessibleResourceIds(SecondOwnerToken, service.ExportInstancePermissionsTopic, client.ListOptions{}, client.Administrate)
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(ids, []string{exportInstance2.ID.String()}) {
			t.Error(ids)
			return
		}
	})

	t.Run("admin delete instance", func(t *testing.T) {
		var exportInstance lib.Instance
		temp, err := json.Marshal(lib.ServingRequest{
			FilterType:       "deviceId",
			Name:             "instanceDelete2",
			Filter:           "device1",
			EntityName:       "device1",
			ServiceName:      "service1",
			Description:      "foo",
			Topic:            "?",
			TimePath:         "?",
			TimePrecision:    "?",
			Generated:        false,
			Offset:           "?",
			ForceUpdate:      false,
			Values:           nil,
			ExportDatabaseID: exportDatabasePublic.ID,
			TimestampFormat:  "?",
		})
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:"+serverPort+"/instance", bytes.NewReader(temp))
		if err != nil {
			t.Error(err)
			return
		}
		exportInstance, err, _ = doReq[lib.Instance](SecondOwnerToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		ids, err, _ := permV2.ListAccessibleResourceIds(SecondOwnerToken, service.ExportInstancePermissionsTopic, client.ListOptions{}, client.Administrate)
		if err != nil {
			t.Error(err)
			return
		}
		sort.Strings(ids)
		expected := []string{exportInstance2.ID.String(), exportInstance.ID.String()}
		sort.Strings(expected)
		if !reflect.DeepEqual(ids, expected) {
			t.Error(ids)
			return
		}

		req, err = http.NewRequest(http.MethodDelete, "http://localhost:"+serverPort+"/admin/instance/"+url.PathEscape(exportInstance.ID.String()), nil)
		if err != nil {
			t.Error(err)
			return
		}
		err, _ = doVoid(client.InternalAdminToken, req)
		if err != nil {
			t.Error(err)
			return
		}

		ids, err, _ = permV2.ListAccessibleResourceIds(SecondOwnerToken, service.ExportInstancePermissionsTopic, client.ListOptions{}, client.Administrate)
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(ids, []string{exportInstance2.ID.String()}) {
			t.Error(ids)
			return
		}
	})

	t.Run("allow testUser access to exportInstance2", func(t *testing.T) {
		_, err, _ = permV2.SetPermission(
			client.InternalAdminToken,
			service.ExportInstancePermissionsTopic,
			exportInstance2.ID.String(),
			client.ResourcePermissions{
				UserPermissions: map[string]model.PermissionsMap{
					TestTokenUser:        {Read: true, Write: true, Execute: true, Administrate: true},
					SecendOwnerTokenUser: {Read: true, Write: true, Execute: true, Administrate: true}},
				GroupPermissions: map[string]model.PermissionsMap{},
				RolePermissions:  map[string]model.PermissionsMap{},
			},
		)
		if err != nil {
			t.Error(err)
			return
		}
	})

	t.Run("update", func(t *testing.T) {
		t.Run("not allowed", func(t *testing.T) {
			temp, err := json.Marshal(lib.ServingRequest{
				FilterType:       "deviceId",
				Name:             "instance1 update",
				Filter:           "device1",
				EntityName:       "device1",
				ServiceName:      "service1",
				Description:      "foo",
				Topic:            "?",
				TimePath:         "?",
				TimePrecision:    "?",
				Generated:        false,
				Offset:           "?",
				ForceUpdate:      false,
				Values:           nil,
				ExportDatabaseID: exportDatabasePublic.ID,
				TimestampFormat:  "?",
			})
			if err != nil {
				t.Error(err)
				return
			}
			req, err := http.NewRequest(http.MethodPut, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance1.ID.String()), bytes.NewReader(temp))
			if err != nil {
				t.Error(err)
				return
			}
			_, err, _ = doReq[lib.Instance](SecondOwnerToken, req)
			if err == nil {
				t.Error("expect error")
				return
			}
		})

		t.Run("allowed", func(t *testing.T) {
			temp, err := json.Marshal(lib.ServingRequest{
				FilterType:       "deviceId",
				Name:             "instance2 update",
				Filter:           "device1",
				EntityName:       "device1",
				ServiceName:      "service1",
				Description:      "foo",
				Topic:            "?",
				TimePath:         "?",
				TimePrecision:    "?",
				Generated:        false,
				Offset:           "?",
				ForceUpdate:      false,
				Values:           nil,
				ExportDatabaseID: exportDatabasePublic.ID,
				TimestampFormat:  "?",
			})
			if err != nil {
				t.Error(err)
				return
			}
			req, err := http.NewRequest(http.MethodPut, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance2.ID.String()), bytes.NewReader(temp))
			if err != nil {
				t.Error(err)
				return
			}
			_, err, _ = doReq[lib.Instance](TestToken, req)
			if err != nil {
				t.Error(err)
				return
			}

			req, err = http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance2.ID.String()), nil)
			if err != nil {
				t.Error(err)
				return
			}
			instance, err, _ := doReq[lib.Instance](TestToken, req)
			if err != nil {
				t.Error(err)
				return
			}
			if instance.Name != "instance2 update" {
				t.Error(instance.Name)
				return
			}
		})
	})

	t.Run("admin list", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/admin/instance", nil)
		if err != nil {
			t.Error(err)
			return
		}
		instances, err, _ := doReq[[]lib.Instance](client.InternalAdminToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		ids := []string{}
		for _, instance := range instances {
			ids = append(ids, instance.ID.String())
		}
		sort.Strings(ids)
		expected := []string{exportInstance1.ID.String(), exportInstance2.ID.String()}
		sort.Strings(expected)
		if !reflect.DeepEqual(ids, expected) {
			t.Error(ids)
			return
		}
	})

	t.Run("list", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance", nil)
		if err != nil {
			t.Error(err)
			return
		}
		instances, err, _ := doReq[lib.InstancesResponse](SecondOwnerToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		ids := []string{}
		for _, instance := range instances.Instances {
			ids = append(ids, instance.ID.String())
		}
		if !reflect.DeepEqual(ids, []string{exportInstance2.ID.String()}) {
			t.Error(ids)
			return
		}

		req, err = http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance", nil)
		if err != nil {
			t.Error(err)
			return
		}
		instances, err, _ = doReq[lib.InstancesResponse](TestToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		ids = []string{}
		for _, instance := range instances.Instances {
			ids = append(ids, instance.ID.String())
		}
		sort.Strings(ids)
		expected := []string{exportInstance1.ID.String(), exportInstance2.ID.String()}
		sort.Strings(expected)
		if !reflect.DeepEqual(ids, expected) {
			t.Error(ids)
			return
		}
	})

	t.Run("get", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance1.ID.String()), nil)
		if err != nil {
			t.Error(err)
			return
		}
		instance, err, _ := doReq[lib.Instance](TestToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		if instance.ID.String() != exportInstance1.ID.String() {
			t.Error(instance.ID.String())
			return
		}

		req, err = http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance2.ID.String()), nil)
		if err != nil {
			t.Error(err)
			return
		}
		instance, err, _ = doReq[lib.Instance](TestToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		if instance.ID.String() != exportInstance2.ID.String() {
			t.Error(instance.ID.String())
			return
		}

		req, err = http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance2.ID.String()), nil)
		if err != nil {
			t.Error(err)
			return
		}
		instance, err, _ = doReq[lib.Instance](SecondOwnerToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		if instance.ID.String() != exportInstance2.ID.String() {
			t.Error(instance.ID.String())
			return
		}

		req, err = http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance1.ID.String()), nil)
		if err != nil {
			t.Error(err)
			return
		}
		instance, err, _ = doReq[lib.Instance](SecondOwnerToken, req)
		if err == nil {
			t.Error("expected error")
			return
		}

	})

	var exportInstance lib.Instance

	t.Run("cleanup", func(t *testing.T) {
		temp, err := json.Marshal(lib.ServingRequest{
			FilterType:       "deviceId",
			Name:             "instanceDelete",
			Filter:           "device1",
			EntityName:       "device1",
			ServiceName:      "service1",
			Description:      "foo",
			Topic:            "?",
			TimePath:         "?",
			TimePrecision:    "?",
			Generated:        false,
			Offset:           "?",
			ForceUpdate:      false,
			Values:           nil,
			ExportDatabaseID: exportDatabasePublic.ID,
			TimestampFormat:  "?",
		})
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:"+serverPort+"/instance", bytes.NewReader(temp))
		if err != nil {
			t.Error(err)
			return
		}
		exportInstance, err, _ = doReq[lib.Instance](SecondOwnerToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		err, _ = permV2.RemoveResource(client.InternalAdminToken, service.ExportInstancePermissionsTopic, exportInstance.ID.String())
		if err != nil {
			t.Error(err)
			return
		}
		_, err, _ = permV2.SetPermission(
			client.InternalAdminToken,
			service.ExportInstancePermissionsTopic,
			"will-be-removed",
			client.ResourcePermissions{
				UserPermissions: map[string]model.PermissionsMap{
					SecendOwnerTokenUser: {Read: true, Write: true, Execute: true, Administrate: true},
				},
				GroupPermissions: map[string]model.PermissionsMap{},
			},
		)
		if err != nil {
			t.Error(err)
			return
		}
		err = serving.ExportInstanceCleanup(time.Second)
		if err != nil {
			t.Error(err)
			return
		}

		ids, err, _ := permV2.AdminListResourceIds(client.InternalAdminToken, service.ExportInstancePermissionsTopic, client.ListOptions{})
		if err != nil {
			t.Error(err)
			return
		}
		sort.Strings(ids)
		expected := []string{exportInstance2.ID.String(), exportInstance1.ID.String(), exportInstance.ID.String()}
		sort.Strings(expected)
		if !reflect.DeepEqual(ids, expected) {
			t.Log("expected:", expected)
			t.Error(ids)
			return
		}

		instances, _, errs := serving.GetInstances("", map[string][]string{}, true, client.InternalAdminToken)
		if len(errs) != 0 {
			t.Error(errs)
			return
		}
		actualIds := []string{}
		for _, instance := range instances {
			actualIds = append(actualIds, instance.ID.String())
		}
		expectedIds := []string{exportInstance1.ID.String(), exportInstance2.ID.String(), exportInstance.ID.String()}
		sort.Strings(actualIds)
		sort.Strings(expectedIds)
		if !reflect.DeepEqual(actualIds, expectedIds) {
			t.Error(actualIds)
			return
		}
	})

	t.Run("list after cleanup", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance", nil)
		if err != nil {
			t.Error(err)
			return
		}
		instances, err, _ := doReq[lib.InstancesResponse](SecondOwnerToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		ids := []string{}
		for _, instance := range instances.Instances {
			ids = append(ids, instance.ID.String())
		}
		expected := []string{exportInstance2.ID.String(), exportInstance.ID.String()}
		sort.Strings(expected)
		sort.Strings(ids)
		if !reflect.DeepEqual(ids, expected) {
			t.Error(ids)
			return
		}

		req, err = http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance", nil)
		if err != nil {
			t.Error(err)
			return
		}
		instances, err, _ = doReq[lib.InstancesResponse](TestToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		ids = []string{}
		for _, instance := range instances.Instances {
			ids = append(ids, instance.ID.String())
		}
		sort.Strings(ids)
		expected = []string{exportInstance1.ID.String(), exportInstance2.ID.String()}
		sort.Strings(expected)
		if !reflect.DeepEqual(ids, expected) {
			t.Error(ids)
			return
		}
	})

	t.Run("get after cleanup", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance1.ID.String()), nil)
		if err != nil {
			t.Error(err)
			return
		}
		instance, err, _ := doReq[lib.Instance](TestToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		if instance.ID.String() != exportInstance1.ID.String() {
			t.Error(instance.ID.String())
			return
		}

		req, err = http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance2.ID.String()), nil)
		if err != nil {
			t.Error(err)
			return
		}
		instance, err, _ = doReq[lib.Instance](TestToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		if instance.ID.String() != exportInstance2.ID.String() {
			t.Error(instance.ID.String())
			return
		}

		req, err = http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance2.ID.String()), nil)
		if err != nil {
			t.Error(err)
			return
		}
		instance, err, _ = doReq[lib.Instance](SecondOwnerToken, req)
		if err != nil {
			t.Error(err)
			return
		}
		if instance.ID.String() != exportInstance2.ID.String() {
			t.Error(instance.ID.String())
			return
		}

		req, err = http.NewRequest(http.MethodGet, "http://localhost:"+serverPort+"/instance/"+url.PathEscape(exportInstance1.ID.String()), nil)
		if err != nil {
			t.Error(err)
			return
		}
		instance, err, _ = doReq[lib.Instance](SecondOwnerToken, req)
		if err == nil {
			t.Error("expected error")
			return
		}

	})

}

func doReq[T any](token string, req *http.Request) (result T, err error, code int) {
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		temp, _ := io.ReadAll(resp.Body) //read error response end ensure that resp.Body is read to EOF
		return result, fmt.Errorf("unexpected statuscode %v: %v", resp.StatusCode, string(temp)), resp.StatusCode
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		_, _ = io.ReadAll(resp.Body) //ensure resp.Body is read to EOF
		return result, err, http.StatusInternalServerError
	}
	return result, nil, resp.StatusCode
}

func doVoid(token string, req *http.Request) (err error, code int) {
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		temp, _ := io.ReadAll(resp.Body) //read error response end ensure that resp.Body is read to EOF
		return fmt.Errorf("unexpected statuscode %v: %v", resp.StatusCode, string(temp)), resp.StatusCode
	}
	return nil, resp.StatusCode
}

const TestTokenUser = "testOwner"
const TestToken = `Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJqdGkiOiIwOGM0N2E4OC0yYzc5LTQyMGYtODEwNC02NWJkOWViYmU0MWUiLCJleHAiOjE1NDY1MDcyMzMsIm5iZiI6MCwiaWF0IjoxNTQ2NTA3MTczLCJpc3MiOiJodHRwOi8vbG9jYWxob3N0OjgwMDEvYXV0aC9yZWFsbXMvbWFzdGVyIiwiYXVkIjoiZnJvbnRlbmQiLCJzdWIiOiJ0ZXN0T3duZXIiLCJ0eXAiOiJCZWFyZXIiLCJhenAiOiJmcm9udGVuZCIsIm5vbmNlIjoiOTJjNDNjOTUtNzViMC00NmNmLTgwYWUtNDVkZDk3M2I0YjdmIiwiYXV0aF90aW1lIjoxNTQ2NTA3MDA5LCJzZXNzaW9uX3N0YXRlIjoiNWRmOTI4ZjQtMDhmMC00ZWI5LTliNjAtM2EwYWUyMmVmYzczIiwiYWNyIjoiMCIsImFsbG93ZWQtb3JpZ2lucyI6WyIqIl0sInJlYWxtX2FjY2VzcyI6eyJyb2xlcyI6WyJ1c2VyIl19LCJyZXNvdXJjZV9hY2Nlc3MiOnsibWFzdGVyLXJlYWxtIjp7InJvbGVzIjpbInZpZXctcmVhbG0iLCJ2aWV3LWlkZW50aXR5LXByb3ZpZGVycyIsIm1hbmFnZS1pZGVudGl0eS1wcm92aWRlcnMiLCJpbXBlcnNvbmF0aW9uIiwiY3JlYXRlLWNsaWVudCIsIm1hbmFnZS11c2VycyIsInF1ZXJ5LXJlYWxtcyIsInZpZXctYXV0aG9yaXphdGlvbiIsInF1ZXJ5LWNsaWVudHMiLCJxdWVyeS11c2VycyIsIm1hbmFnZS1ldmVudHMiLCJtYW5hZ2UtcmVhbG0iLCJ2aWV3LWV2ZW50cyIsInZpZXctdXNlcnMiLCJ2aWV3LWNsaWVudHMiLCJtYW5hZ2UtYXV0aG9yaXphdGlvbiIsIm1hbmFnZS1jbGllbnRzIiwicXVlcnktZ3JvdXBzIl19LCJhY2NvdW50Ijp7InJvbGVzIjpbIm1hbmFnZS1hY2NvdW50IiwibWFuYWdlLWFjY291bnQtbGlua3MiLCJ2aWV3LXByb2ZpbGUiXX19LCJyb2xlcyI6WyJ1c2VyIl19.ykpuOmlpzj75ecSI6cHbCATIeY4qpyut2hMc1a67Ycg`

const SecendOwnerTokenUser = "secondOwner"
const SecondOwnerToken = `Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJqdGkiOiIwOGM0N2E4OC0yYzc5LTQyMGYtODEwNC02NWJkOWViYmU0MWUiLCJleHAiOjE1NDY1MDcyMzMsIm5iZiI6MCwiaWF0IjoxNTQ2NTA3MTczLCJpc3MiOiJodHRwOi8vbG9jYWxob3N0OjgwMDEvYXV0aC9yZWFsbXMvbWFzdGVyIiwiYXVkIjoiZnJvbnRlbmQiLCJzdWIiOiJzZWNvbmRPd25lciIsInR5cCI6IkJlYXJlciIsImF6cCI6ImZyb250ZW5kIiwibm9uY2UiOiI5MmM0M2M5NS03NWIwLTQ2Y2YtODBhZS00NWRkOTczYjRiN2YiLCJhdXRoX3RpbWUiOjE1NDY1MDcwMDksInNlc3Npb25fc3RhdGUiOiI1ZGY5MjhmNC0wOGYwLTRlYjktOWI2MC0zYTBhZTIyZWZjNzMiLCJhY3IiOiIwIiwiYWxsb3dlZC1vcmlnaW5zIjpbIioiXSwicmVhbG1fYWNjZXNzIjp7InJvbGVzIjpbInVzZXIiXX0sInJlc291cmNlX2FjY2VzcyI6eyJtYXN0ZXItcmVhbG0iOnsicm9sZXMiOlsidmlldy1yZWFsbSIsInZpZXctaWRlbnRpdHktcHJvdmlkZXJzIiwibWFuYWdlLWlkZW50aXR5LXByb3ZpZGVycyIsImltcGVyc29uYXRpb24iLCJjcmVhdGUtY2xpZW50IiwibWFuYWdlLXVzZXJzIiwicXVlcnktcmVhbG1zIiwidmlldy1hdXRob3JpemF0aW9uIiwicXVlcnktY2xpZW50cyIsInF1ZXJ5LXVzZXJzIiwibWFuYWdlLWV2ZW50cyIsIm1hbmFnZS1yZWFsbSIsInZpZXctZXZlbnRzIiwidmlldy11c2VycyIsInZpZXctY2xpZW50cyIsIm1hbmFnZS1hdXRob3JpemF0aW9uIiwibWFuYWdlLWNsaWVudHMiLCJxdWVyeS1ncm91cHMiXX0sImFjY291bnQiOnsicm9sZXMiOlsibWFuYWdlLWFjY291bnQiLCJtYW5hZ2UtYWNjb3VudC1saW5rcyIsInZpZXctcHJvZmlsZSJdfX0sInJvbGVzIjpbInVzZXIiXX0.cq8YeUuR0jSsXCEzp634fTzNbGkq_B8KbVrwBPgceJ4`
