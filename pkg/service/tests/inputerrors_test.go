/*
 * Copyright 2026 InfAI (CC SES)
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

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/SENERGY-Platform/analytics-serving/lib"
	"github.com/SENERGY-Platform/analytics-serving/pkg/api"
	"github.com/SENERGY-Platform/analytics-serving/pkg/config"
	"github.com/SENERGY-Platform/analytics-serving/pkg/db"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service/tests/mocks"
	"github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/SENERGY-Platform/permissions-v2/pkg/model"
	"github.com/google/uuid"
)

// countingDriver counts CreateInstance calls, so a test can show that a rejected request never reached the export worker.
type countingDriver struct {
	mocks.Driver
	created atomic.Int64
}

func (d *countingDriver) CreateInstance(instance *lib.Instance, dataFields string, tagFields string) (string, error) {
	d.created.Add(1)
	return d.Driver.CreateInstance(instance, dataFields, tagFields)
}

// pipelineStub grants every pipeline except "denied" (no access) and "broken" (the check itself fails).
type pipelineStub struct{}

func (pipelineStub) UserHasPipelineAccess(id string, _ string) (bool, error) {
	switch id {
	case "denied":
		return false, nil
	case "broken":
		return false, errors.New("pipeline registry unavailable")
	}
	return true, nil
}

const messageSourceAccessDenied = "no access to the source of the export"

func TestCallerErrorsOnCreateAndUpdate(t *testing.T) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := startPostgres(t, ctx, wg)
	if err := db.Init(&cfg); err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.NewMigration(db.GetDB(), "").Migrate(); err != nil {
		t.Fatal(err)
	}

	permV2, err := client.NewTestClient(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err, _ = permV2.SetTopic(client.InternalAdminToken, client.Topic{Id: service.PermV2DeviceTopic})
	if err != nil {
		t.Fatal(err)
	}
	for device, owner := range map[string]string{"device1": TestTokenUser, "device2": SecendOwnerTokenUser} {
		_, err, _ = permV2.SetPermission(client.InternalAdminToken, service.PermV2DeviceTopic, device, client.ResourcePermissions{
			UserPermissions:  map[string]model.PermissionsMap{owner: {Read: true, Write: true, Execute: true, Administrate: true}},
			GroupPermissions: map[string]model.PermissionsMap{},
			RolePermissions:  map[string]model.PermissionsMap{},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	driver := &countingDriver{}
	var (
		drv      service.Driver              = driver
		pipeline service.PipelineApiService  = pipelineStub{}
		imports  service.ImportDeployService = mocks.Imports{}
		perm     client.Client               = permV2
		influx   service.Influx              = mocks.Influx{}
	)
	router, err := api.CreateServer(&config.Config{Driver: "mock", CleanupConfig: config.CleanupConfig{WaitDuration: "1s"}},
		&drv, &pipeline, &imports, &perm, &influx)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()

	send := func(t *testing.T, method string, path string, token string, body any) (int, []byte) {
		t.Helper()
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, raw
	}

	createDatabase := func(t *testing.T, token string, dbType string, public bool) lib.ExportDatabase {
		t.Helper()
		status, raw := send(t, http.MethodPost, "/databases", token, lib.ExportDatabaseRequest{
			Name: dbType, Type: dbType, Deployment: "internal", Url: "?", EwFilterTopic: "?", Public: public,
		})
		if status != http.StatusOK {
			t.Fatalf("create database: status %d, body %s", status, raw)
		}
		var database lib.ExportDatabase
		if err := json.Unmarshal(raw, &database); err != nil {
			t.Fatal(err)
		}
		return database
	}
	influxDatabase := createDatabase(t, TestToken, "influxdb", false)
	timescaleDatabase := createDatabase(t, TestToken, "timescaledb", false)
	foreignDatabase := createDatabase(t, SecondOwnerToken, "influxdb", false)

	validRequest := func() lib.ServingRequest {
		return lib.ServingRequest{
			FilterType:       "deviceId",
			Filter:           "device1",
			Name:             "export",
			EntityName:       "entity",
			ServiceName:      "service",
			Topic:            "topic",
			Offset:           "largest",
			ExportDatabaseID: influxDatabase.ID,
		}
	}

	// state is the number of stored instances, permission resources of instances and export worker creates.
	type state struct{ instances, permissions, created int64 }
	current := func(t *testing.T) state {
		t.Helper()
		var instances int64
		if errs := db.DB.Model(&lib.Instance{}).Count(&instances).GetErrors(); len(errs) > 0 {
			t.Fatal(errs)
		}
		ids, err, _ := permV2.AdminListResourceIds(client.InternalAdminToken, service.ExportInstancePermissionsTopic, client.ListOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return state{instances: instances, permissions: int64(len(ids)), created: driver.created.Load()}
	}
	assertUnchanged := func(t *testing.T, before state) {
		t.Helper()
		if after := current(t); after != before {
			t.Errorf("state changed from %+v to %+v, want nothing created", before, after)
		}
	}

	validationErrors := func(t *testing.T, status int, raw []byte) map[string][]string {
		t.Helper()
		if status != http.StatusBadRequest {
			t.Fatalf("status %d, body %s, want 400", status, raw)
		}
		var body map[string]map[string][]string
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("body %s: %v", raw, err)
		}
		return body["validationErrors"]
	}
	assertFields := func(t *testing.T, errs map[string][]string, want ...string) {
		t.Helper()
		if len(errs) != len(want) {
			t.Errorf("validation errors %v, want exactly the fields %v", errs, want)
		}
		for _, field := range want {
			if len(errs[field]) == 0 {
				t.Errorf("validation errors %v, want an entry for %s", errs, field)
			}
		}
	}
	assertForbidden := func(t *testing.T, status int, raw []byte) {
		t.Helper()
		if status != http.StatusForbidden || string(raw) != messageSourceAccessDenied {
			t.Errorf("status %d, body %q, want 403 %q", status, raw, messageSourceAccessDenied)
		}
	}
	create := func(t *testing.T, request lib.ServingRequest) (int, []byte) {
		t.Helper()
		return send(t, http.MethodPost, "/instance", TestToken, request)
	}

	t.Run("unknown filter type", func(t *testing.T) {
		before := current(t)
		request := validRequest()
		request.FilterType = "foo"
		status, raw := create(t, request)
		assertFields(t, validationErrors(t, status, raw), "FilterType")
		assertUnchanged(t, before)
	})

	t.Run("description at the column limit", func(t *testing.T) {
		request := validRequest()
		request.Description = strings.Repeat("ä", 255)
		status, raw := create(t, request)
		if status != http.StatusCreated {
			t.Fatalf("status %d, body %s, want 201", status, raw)
		}
		var instance lib.Instance
		if err := json.Unmarshal(raw, &instance); err != nil {
			t.Fatal(err)
		}
		var stored lib.Instance
		if errs := db.DB.Where("id = ?", instance.ID.String()).First(&stored).GetErrors(); len(errs) > 0 {
			t.Fatal(errs)
		}
		if stored.Description != request.Description {
			t.Errorf("stored description has %d characters, want the 255 sent", len([]rune(stored.Description)))
		}
	})

	t.Run("description beyond the column limit", func(t *testing.T) {
		before := current(t)
		request := validRequest()
		request.Description = strings.Repeat("ä", 256)
		status, raw := create(t, request)
		assertFields(t, validationErrors(t, status, raw), "Description")
		assertUnchanged(t, before)
	})

	t.Run("value name beyond the column limit", func(t *testing.T) {
		request := validRequest()
		request.Values = []lib.ServingRequestValue{{Name: "a", Type: "float", Path: "a"}, {Name: strings.Repeat("x", 256), Type: "float", Path: "b"}}
		status, raw := create(t, request)
		assertFields(t, validationErrors(t, status, raw), "Values[1].Name")
	})

	t.Run("device of another user", func(t *testing.T) {
		before := current(t)
		request := validRequest()
		request.Filter = "device2"
		status, raw := create(t, request)
		assertForbidden(t, status, raw)
		assertUnchanged(t, before)
	})

	t.Run("unknown device", func(t *testing.T) {
		request := validRequest()
		request.Filter = uuid.NewString()
		status, raw := create(t, request)
		assertForbidden(t, status, raw)
	})

	t.Run("pipeline without access", func(t *testing.T) {
		before := current(t)
		request := validRequest()
		request.FilterType, request.Filter = "operatorId", "denied:operator"
		status, raw := create(t, request)
		assertForbidden(t, status, raw)
		assertUnchanged(t, before)
	})

	t.Run("failing pipeline check stays a server error", func(t *testing.T) {
		request := validRequest()
		request.FilterType, request.Filter = "operatorId", "broken:operator"
		status, raw := create(t, request)
		if status != http.StatusInternalServerError {
			t.Errorf("status %d, body %s, want 500", status, raw)
		}
	})

	t.Run("timescale database without timestamp format", func(t *testing.T) {
		before := current(t)
		request := validRequest()
		request.ExportDatabaseID, request.TimePath = timescaleDatabase.ID, "time"
		status, raw := create(t, request)
		assertFields(t, validationErrors(t, status, raw), "TimestampFormat")
		assertUnchanged(t, before)
	})

	t.Run("timescale database without time path and timestamp format", func(t *testing.T) {
		before := current(t)
		request := validRequest()
		request.ExportDatabaseID = timescaleDatabase.ID
		status, raw := create(t, request)
		assertFields(t, validationErrors(t, status, raw), "TimePath", "TimestampFormat")
		assertUnchanged(t, before)
	})

	t.Run("timescale database with time path and timestamp format", func(t *testing.T) {
		request := validRequest()
		request.ExportDatabaseID, request.TimePath, request.TimestampFormat = timescaleDatabase.ID, "time", "unix"
		if status, raw := create(t, request); status != http.StatusCreated {
			t.Errorf("status %d, body %s, want 201", status, raw)
		}
	})

	for name, databaseId := range map[string]string{
		"unknown export database":                 uuid.NewString(),
		"private export database of another user": foreignDatabase.ID,
		"missing export database":                 "",
	} {
		t.Run(name, func(t *testing.T) {
			before := current(t)
			request := validRequest()
			request.ExportDatabaseID = databaseId
			status, raw := create(t, request)
			assertFields(t, validationErrors(t, status, raw), "ExportDatabaseID")
			assertUnchanged(t, before)
		})
	}

	t.Run("update to a device of another user", func(t *testing.T) {
		status, raw := create(t, validRequest())
		if status != http.StatusCreated {
			t.Fatalf("status %d, body %s, want 201", status, raw)
		}
		var instance lib.Instance
		if err := json.Unmarshal(raw, &instance); err != nil {
			t.Fatal(err)
		}
		before := current(t)
		request := validRequest()
		request.Filter = "device2"
		status, raw = send(t, http.MethodPut, "/instance/"+instance.ID.String(), TestToken, request)
		assertForbidden(t, status, raw)
		assertUnchanged(t, before)
		var stored lib.Instance
		if errs := db.DB.Where("id = ?", instance.ID.String()).First(&stored).GetErrors(); len(errs) > 0 {
			t.Fatal(errs)
		}
		if stored.Filter != "device1" {
			t.Errorf("stored filter = %q, want the unchanged device1", stored.Filter)
		}
	})
}
