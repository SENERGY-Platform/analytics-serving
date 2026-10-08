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
	"errors"
	"fmt"
	"strconv"
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

func startPostgres(t *testing.T, ctx context.Context, wg *sync.WaitGroup) config.PostgresConfig {
	t.Helper()
	_, dbIp, _, err := docker.PostgresWithNetwork(ctx, wg, "exports")
	if err != nil {
		t.Fatal(err)
	}
	util.InitStructLogger("warn")
	return config.PostgresConfig{Host: dbIp, Port: docker.PostgresPort, User: docker.PostgresUser, Password: docker.PostgresPassword, Database: "exports", MaxOpenConns: 8, MaxIdleConns: 4}
}

func assertForeignKeys(t *testing.T, when string) {
	t.Helper()
	for _, table := range []string{"instances", "values"} {
		var count int
		err := db.GetDB().DB().QueryRow(`SELECT count(*) FROM information_schema.table_constraints
			WHERE table_schema = current_schema() AND table_name = $1 AND constraint_type = 'FOREIGN KEY'`, table).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("%s: table %s has %d foreign keys, want 1", when, table, count)
		}
	}
}

func TestStartTwiceOnSameDatabase(t *testing.T) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := startPostgres(t, ctx, wg)

	migrationInfo := `{"id": "migrated", "name": "migrated", "type": "influxdb", "deployment": "internal", "user_id": "owner", "public": true}`
	for _, start := range []string{"first start", "second start"} {
		if err := db.Init(&cfg); err != nil {
			t.Fatal(start, err)
		}
		m := db.NewMigration(db.GetDB(), migrationInfo)
		if err := m.Migrate(); err != nil {
			t.Fatalf("%s: %v", start, err)
		}
		if err := m.TmpMigrate(); err != nil {
			t.Fatalf("%s: %v", start, err)
		}
		assertForeignKeys(t, start)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("concurrent starts", func(t *testing.T) {
		if err := db.Init(&cfg); err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		// Each round removes the foreign keys and lets several instances add them at once, which makes some
		// of them fail on a constraint another one has just added.
		for round := range 5 {
			dropForeignKeys(t)
			const starts = 16
			errs := make([]error, starts)
			begin := make(chan struct{})
			var started sync.WaitGroup
			for i := range starts {
				started.Add(1)
				go func() {
					defer started.Done()
					<-begin
					errs[i] = db.NewMigration(db.GetDB(), "").Migrate()
				}()
			}
			close(begin)
			started.Wait()
			if err := errors.Join(errs...); err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
			assertForeignKeys(t, "after concurrent starts")
		}
	})
}

func dropForeignKeys(t *testing.T) {
	t.Helper()
	rows, err := db.GetDB().DB().Query(`SELECT table_name, constraint_name FROM information_schema.table_constraints
		WHERE table_schema = current_schema() AND constraint_type = 'FOREIGN KEY'`)
	if err != nil {
		t.Fatal(err)
	}
	var drops []string
	for rows.Next() {
		var table, constraint string
		if err = rows.Scan(&table, &constraint); err != nil {
			t.Fatal(err)
		}
		drops = append(drops, `ALTER TABLE "`+table+`" DROP CONSTRAINT "`+constraint+`"`)
	}
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatal(err)
	}
	for _, drop := range drops {
		if _, err = db.GetDB().DB().Exec(drop); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSearchIgnoresCase(t *testing.T) {
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
	_, err, _ = permV2.SetPermission(client.InternalAdminToken, service.PermV2DeviceTopic, "device1", client.ResourcePermissions{
		UserPermissions:  map[string]model.PermissionsMap{TestTokenUser: {Read: true, Write: true, Execute: true, Administrate: true}},
		GroupPermissions: map[string]model.PermissionsMap{},
		RolePermissions:  map[string]model.PermissionsMap{},
	})
	if err != nil {
		t.Fatal(err)
	}
	serving, err := service.NewServing(mocks.Driver{}, mocks.Influx{}, mocks.Pipeline{}, mocks.Imports{}, "", permV2, "", time.Second)
	if err != nil {
		t.Fatal(err)
	}

	database, errs := serving.CreateExportDatabase("", lib.ExportDatabaseRequest{
		Name:          "lowercase database",
		Type:          "influxdb",
		Deployment:    "internal",
		Url:           "?",
		EwFilterTopic: "?",
	}, TestTokenUser)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	instance, err := serving.CreateInstance(lib.ServingRequest{
		FilterType:       "deviceId",
		Filter:           "device1",
		Name:             "lowercase export",
		Description:      "lowercase description",
		EntityName:       "lowercase entity",
		ServiceName:      "lowercase service",
		Topic:            "?",
		Offset:           "?",
		Generated:        true,
		ExportDatabaseID: database.ID,
		Values:           []lib.ServingRequestValue{{Name: "a", Type: "float", Path: "a"}, {Name: "b", Type: "string", Path: "b", Tag: true}},
	}, TestTokenUser, TestToken)
	if err != nil {
		t.Fatal(err)
	}

	instanceCases := []struct {
		args map[string][]string
		want int
	}{
		{map[string][]string{"search": {"LowerCase EXPORT"}}, 1},
		{map[string][]string{"search": {"name:LOWERCASE"}}, 1},
		{map[string][]string{"search": {"description:Description"}}, 1},
		{map[string][]string{"search": {"entity_name:ENTITY"}}, 1},
		{map[string][]string{"search": {"service_name:Service"}}, 1},
		{map[string][]string{"search": {"UPPERCASE"}}, 0},
		{map[string][]string{"generated": {"true"}}, 1},
		{map[string][]string{"generated": {"false"}}, 0},
		{map[string][]string{"export_database_id": {database.ID}}, 1},
		{map[string][]string{"internal_only": {"true"}}, 1},
		{map[string][]string{"order": {"offset:desc"}}, 1},
	}
	for _, c := range instanceCases {
		instances, total, errs := serving.GetInstances(TestTokenUser, c.args, true, "")
		if len(errs) > 0 {
			t.Errorf("instances %v: %v", c.args, errs)
			continue
		}
		if len(instances) != c.want || total != int64(c.want) {
			t.Errorf("instances %v: got %d of total %d, want %d", c.args, len(instances), total, c.want)
			continue
		}
		if c.want == 1 && (instances[0].ID != instance.ID || len(instances[0].Values) != 2) {
			t.Errorf("instances %v: got %v with %d values, want %v with 2", c.args, instances[0].ID, len(instances[0].Values), instance.ID)
		}
	}

	databaseCases := []struct {
		args map[string][]string
		want int
	}{
		{map[string][]string{"search": {"LOWERCASE"}}, 1},
		{map[string][]string{"search": {"type:INFLUX"}}, 1},
		{map[string][]string{"search": {"UPPERCASE"}}, 0},
		{map[string][]string{"deployment": {"internal"}}, 1},
		{map[string][]string{"public": {"false"}}, 1},
		{map[string][]string{"public": {"true"}}, 0},
		{map[string][]string{"owner": {"true"}}, 1},
		{map[string][]string{"owner": {"false"}}, 0},
		{map[string][]string{"order": {"type:asc"}}, 1},
	}
	for _, c := range databaseCases {
		databases, errs := serving.GetExportDatabases(TestTokenUser, c.args)
		if len(errs) > 0 {
			t.Errorf("databases %v: %v", c.args, errs)
			continue
		}
		if len(databases) != c.want {
			t.Errorf("databases %v: got %d, want %d", c.args, len(databases), c.want)
		}
	}

	// "values" is a reserved word; deleting the instance must reach its rows through the cascading foreign key.
	deleted, errs := serving.DeleteInstanceWithPermHandling(instance.ID.String(), "", true, client.InternalAdminToken)
	if !deleted || len(errs) > 0 {
		t.Fatal(deleted, errs)
	}
	var values int
	if err = db.GetDB().DB().QueryRow(`SELECT count(*) FROM "values"`).Scan(&values); err != nil {
		t.Fatal(err)
	}
	if values != 0 {
		t.Errorf("%d values left after deleting their instance", values)
	}
}

// A new or emptied database must not make the cleanup drop every export permission, including shares with
// groups and roles that nothing could restore.
func TestCleanupKeepsPermissionsOfEmptyDatabase(t *testing.T) {
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
	_, err, _ = permV2.SetTopic(client.InternalAdminToken, client.Topic{Id: service.ExportInstancePermissionsTopic})
	if err != nil {
		t.Fatal(err)
	}
	shared := client.ResourcePermissions{
		UserPermissions:  map[string]model.PermissionsMap{TestTokenUser: {Read: true, Write: true, Execute: true, Administrate: true}},
		GroupPermissions: map[string]model.PermissionsMap{"group1": {Read: true}},
		RolePermissions:  map[string]model.PermissionsMap{"role1": {Read: true, Execute: true}},
	}
	for _, id := range []string{"export-a", "export-b"} {
		if _, err, _ = permV2.SetPermission(client.InternalAdminToken, service.ExportInstancePermissionsTopic, id, shared); err != nil {
			t.Fatal(err)
		}
	}

	// NewServing runs the startup cleanup; the second call is what the cron job runs.
	serving, err := service.NewServing(mocks.Driver{}, mocks.Influx{}, mocks.Pipeline{}, mocks.Imports{}, "", permV2, "", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if err = serving.ExportInstanceCleanup(time.Millisecond); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"export-a", "export-b"} {
		resource, err, _ := permV2.GetResource(client.InternalAdminToken, service.ExportInstancePermissionsTopic, id)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		if len(resource.GroupPermissions) != 1 || len(resource.RolePermissions) != 1 || len(resource.UserPermissions) != 1 {
			t.Errorf("%s: permissions changed to %+v", id, resource.ResourcePermissions)
		}
	}
}

// Paging through rows with equal sort values must return every row exactly once.
func TestPagingIsStable(t *testing.T) {
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

	const rows = 40
	for i := range rows {
		database := lib.ExportDatabase{ID: fmt.Sprintf("db-%02d", (i*17)%rows), Name: "same", Type: "same", Public: true}
		if err := db.GetDB().Create(&database).Error; err != nil {
			t.Fatal(err)
		}
		instance := lib.Instance{ID: uuid.New(), Name: "same", FilterType: "same", ExportDatabaseID: database.ID}
		if err := db.GetDB().Create(&instance).Error; err != nil {
			t.Fatal(err)
		}
	}
	serving, err := service.NewServing(mocks.Driver{}, mocks.Influx{}, mocks.Pipeline{}, mocks.Imports{}, "", nil, "", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	const pageSize = 3
	for _, order := range []string{"filter_type:asc", "name:desc", ""} {
		seenInstances := map[string]int{}
		seenDatabases := map[string]int{}
		for offset := 0; offset < rows; offset += pageSize {
			args := map[string][]string{"limit": {strconv.Itoa(pageSize)}, "offset": {strconv.Itoa(offset)}}
			if order != "" {
				args["order"] = []string{order}
			}
			instances, _, errs := serving.GetInstances("", args, true, "")
			if len(errs) > 0 {
				t.Fatal(order, errs)
			}
			for _, instance := range instances {
				seenInstances[instance.ID.String()]++
			}
			if order == "name:desc" {
				databases, errs := serving.GetExportDatabases("", args)
				if len(errs) > 0 {
					t.Fatal(order, errs)
				}
				for _, database := range databases {
					seenDatabases[database.ID]++
				}
			}
		}
		checkOnce(t, "instances ordered by "+order, seenInstances, rows)
		if order == "name:desc" {
			checkOnce(t, "export databases ordered by "+order, seenDatabases, rows)
		}
	}
}

func checkOnce(t *testing.T, what string, seen map[string]int, want int) {
	t.Helper()
	repeated := 0
	for _, n := range seen {
		if n > 1 {
			repeated++
		}
	}
	if len(seen) != want || repeated > 0 {
		t.Errorf("%s: %d of %d rows seen, %d of them more than once", what, len(seen), want, repeated)
	}
}
