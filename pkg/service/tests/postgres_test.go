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
