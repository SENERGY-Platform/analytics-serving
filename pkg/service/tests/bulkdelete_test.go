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
	"context"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SENERGY-Platform/analytics-serving/lib"
	"github.com/SENERGY-Platform/analytics-serving/pkg/config"
	"github.com/SENERGY-Platform/analytics-serving/pkg/db"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service/tests/docker"
	"github.com/SENERGY-Platform/analytics-serving/pkg/service/tests/mocks"
	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	"github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/SENERGY-Platform/permissions-v2/pkg/model"
	"github.com/google/uuid"
)

func TestDeleteInstancesForUser(t *testing.T) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, dbIp, _, err := docker.MySqlWithNetwork(ctx, wg, "exports")
	if err != nil {
		t.Fatal(err)
	}
	util.InitStructLogger("warn")
	err = db.Init(&config.MySQLConfig{Host: dbIp, Port: 3306, User: "usr", Password: "pw", Database: "exports", MaxOpenConns: 8, MaxIdleConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := db.GetDB().DB().Stats().MaxOpenConnections; got != 8 {
		t.Fatalf("pool max open connections = %d, want 8 from the config", got)
	}
	db.NewMigration(db.GetDB(), "").Migrate()

	permV2, err := client.NewTestClient(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err, _ = permV2.SetTopic(client.InternalAdminToken, client.Topic{Id: service.PermV2DeviceTopic})
	if err != nil {
		t.Fatal(err)
	}
	_, err, _ = permV2.SetPermission(client.InternalAdminToken, service.PermV2DeviceTopic, "device1", client.ResourcePermissions{
		UserPermissions: map[string]model.PermissionsMap{
			TestTokenUser:        {Read: true, Write: true, Execute: true, Administrate: true},
			SecendOwnerTokenUser: {Read: true, Write: true, Execute: true, Administrate: true},
		},
		GroupPermissions: map[string]model.PermissionsMap{},
		RolePermissions:  map[string]model.PermissionsMap{},
	})
	if err != nil {
		t.Fatal(err)
	}

	newServing := func(t *testing.T, driver service.Driver) *service.Serving {
		t.Helper()
		serving, err := service.NewServing(driver, mocks.Influx{}, mocks.Pipeline{}, mocks.Imports{}, "", permV2, "", time.Second)
		if err != nil {
			t.Fatal(err)
		}
		return serving
	}

	exportDatabase, errs := newServing(t, &mocks.RecordingDriver{}).CreateExportDatabase("", lib.ExportDatabaseRequest{
		Name:          "bulk",
		Type:          "?",
		Deployment:    "?",
		Url:           "?",
		EwFilterTopic: "?",
		Public:        true,
	}, SecendOwnerTokenUser)
	if len(errs) > 0 {
		t.Fatal(errs)
	}

	createInstances := func(t *testing.T, serving *service.Serving, n int, userId string, token string) (ids []string) {
		t.Helper()
		for range n {
			instance, err := serving.CreateInstance(lib.ServingRequest{
				FilterType:       "deviceId",
				Name:             "bulk",
				Filter:           "device1",
				EntityName:       "device1",
				ServiceName:      "service1",
				Topic:            "?",
				TimePath:         "?",
				Offset:           "?",
				ExportDatabaseID: exportDatabase.ID,
			}, userId, token)
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, instance.ID.String())
		}
		return ids
	}

	remainingInDb := func(t *testing.T, ids []string) (remaining []string) {
		t.Helper()
		var instances []lib.Instance
		if errs := db.DB.Where("id IN (?)", ids).Find(&instances).GetErrors(); len(errs) > 0 {
			t.Fatal(errs)
		}
		for _, instance := range instances {
			remaining = append(remaining, instance.ID.String())
		}
		slices.Sort(remaining)
		return remaining
	}

	remainingInPermissions := func(t *testing.T, ids []string) (remaining []string) {
		t.Helper()
		known, err, _ := permV2.AdminListResourceIds(client.InternalAdminToken, service.ExportInstancePermissionsTopic, client.ListOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range ids {
			if slices.Contains(known, id) {
				remaining = append(remaining, id)
			}
		}
		slices.Sort(remaining)
		return remaining
	}

	t.Run("deletes every requested instance once and reports repeated ids once", func(t *testing.T) {
		driver := &mocks.RecordingDriver{}
		serving := newServing(t, driver)
		ids := createInstances(t, serving, 3, TestTokenUser, TestToken)
		a, b, c := ids[0], ids[1], ids[2]

		deleted, errs := serving.DeleteInstancesForUser([]string{a, b, a, c, b}, TestTokenUser, TestToken)

		if len(errs) > 0 {
			t.Fatal(errs)
		}
		if want := []string{a, b, c}; !reflect.DeepEqual(deleted, want) {
			t.Errorf("deleted = %v, want %v", deleted, want)
		}
		driverDeleted := driver.Deleted()
		slices.Sort(driverDeleted)
		if want := slices.Sorted(slices.Values(ids)); !reflect.DeepEqual(driverDeleted, want) {
			t.Errorf("driver deletes = %v, want each id exactly once %v", driverDeleted, want)
		}
		if remaining := remainingInDb(t, ids); len(remaining) > 0 {
			t.Errorf("instances left in db: %v", remaining)
		}
		if remaining := remainingInPermissions(t, ids); len(remaining) > 0 {
			t.Errorf("instances left in permissions-v2: %v", remaining)
		}
	})

	t.Run("runs the deletes of one request concurrently", func(t *testing.T) {
		driver := &mocks.RecordingDriver{Delay: 500 * time.Millisecond}
		serving := newServing(t, driver)
		ids := createInstances(t, serving, 12, TestTokenUser, TestToken)

		deleted, errs := serving.DeleteInstancesForUser(ids, TestTokenUser, TestToken)

		if len(errs) > 0 {
			t.Fatal(errs)
		}
		if !reflect.DeepEqual(deleted, ids) {
			t.Errorf("deleted = %v, want %v", deleted, ids)
		}
		if got := driver.MaxInFlight(); got < 2 {
			t.Errorf("max concurrent driver deletes = %d, want more than 1", got)
		}
	})

	t.Run("reports deleted ids and errors in input order on partial failure", func(t *testing.T) {
		driver := &mocks.RecordingDriver{}
		serving := newServing(t, driver)
		own := createInstances(t, serving, 3, TestTokenUser, TestToken)
		failing, slow, fast := own[0], own[1], own[2]
		foreign := createInstances(t, serving, 1, SecendOwnerTokenUser, SecondOwnerToken)[0]
		unknown := uuid.NewString()
		// Completion order differs from input order: the failing delete retries for about 20s, the denied one
		// fails at once and the fast one overtakes the slow one.
		driver.FailIds = map[string]bool{failing: true}
		driver.Delays = map[string]time.Duration{slow: time.Second}

		deleted, errs := serving.DeleteInstancesForUser([]string{failing, slow, foreign, fast, unknown}, TestTokenUser, TestToken)

		if want := []string{slow, fast}; !reflect.DeepEqual(deleted, want) {
			t.Errorf("deleted = %v, want %v", deleted, want)
		}
		if len(errs) != 2 {
			t.Fatalf("errors = %v, want one for the failing and one for the foreign instance", errs)
		}
		if !strings.Contains(errs[0].Error(), "driver failure for "+failing) {
			t.Errorf("errors[0] = %v, want the driver failure of %v", errs[0], failing)
		}
		if errs[1].Error() != "access denied" {
			t.Errorf("errors[1] = %v, want access denied for %v", errs[1], foreign)
		}
		if remaining, want := remainingInDb(t, own), []string{failing}; !reflect.DeepEqual(remaining, want) {
			t.Errorf("own instances left in db = %v, want %v", remaining, want)
		}
		if remaining := remainingInDb(t, []string{foreign}); !reflect.DeepEqual(remaining, []string{foreign}) {
			t.Errorf("foreign instance was deleted")
		}
	})
}
