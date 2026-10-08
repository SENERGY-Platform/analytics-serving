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

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// main logs the whole config as JSON, so the passwords must not survive the marshalling.
func TestConfigJsonHidesPasswords(t *testing.T) {
	t.Setenv("PG_PASSWORD", "postgres-secret-from-env")
	t.Setenv("INFLUX_DB_PASSWORD", "influx-secret-from-env")
	cfg, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Postgres.Password.Value() != "postgres-secret-from-env" || cfg.InfluxConfig.Password.Value() != "influx-secret-from-env" {
		t.Fatalf("passwords not loaded from env: postgres %q, influx %q", cfg.Postgres.Password.Value(), cfg.InfluxConfig.Password.Value())
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"postgres-secret-from-env", "influx-secret-from-env"} {
		if strings.Contains(string(b), secret) {
			t.Errorf("config JSON contains %q: %s", secret, b)
		}
	}
}

func TestConfigFileSetsPasswords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"postgres": {"password": "postgres-secret-from-file"}, "influx_config": {"password": "influx-secret-from-file"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Postgres.Password.Value() != "postgres-secret-from-file" || cfg.InfluxConfig.Password.Value() != "influx-secret-from-file" {
		t.Fatalf("passwords not loaded from file: postgres %q, influx %q", cfg.Postgres.Password.Value(), cfg.InfluxConfig.Password.Value())
	}
}

// Parallel deletes each take a connection, so the pool needs a ceiling by default.
func TestConfigLimitsThePostgresPool(t *testing.T) {
	cfg, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Postgres.MaxOpenConns != 25 || cfg.Postgres.MaxIdleConns != 10 {
		t.Fatalf("default pool limits: open %d, idle %d, want 25 and 10", cfg.Postgres.MaxOpenConns, cfg.Postgres.MaxIdleConns)
	}

	t.Setenv("POSTGRES_MAX_OPEN_CONNS", "40")
	t.Setenv("POSTGRES_MAX_IDLE_CONNS", "5")
	cfg, err = New("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Postgres.MaxOpenConns != 40 || cfg.Postgres.MaxIdleConns != 5 {
		t.Fatalf("pool limits from env: open %d, idle %d, want 40 and 5", cfg.Postgres.MaxOpenConns, cfg.Postgres.MaxIdleConns)
	}
}

// The deployment fills these from the platform's postgres-credentials Secret and its own manifest.
func TestConfigReadsPostgresEnv(t *testing.T) {
	cfg, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Postgres.Port != 5432 {
		t.Fatalf("default port %d, want 5432", cfg.Postgres.Port)
	}

	t.Setenv("POSTGRES_HOST", "pg.example")
	t.Setenv("POSTGRES_PORT", "6432")
	t.Setenv("POSTGRES_DB", "analytics_serving")
	t.Setenv("PG_USER", "serving-user")
	t.Setenv("PG_PASSWORD", "serving-password")
	cfg, err = New("")
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Postgres
	if got.Host != "pg.example" || got.Port != 6432 || got.Database != "analytics_serving" || got.User != "serving-user" || got.Password.Value() != "serving-password" {
		t.Fatalf("postgres config from env: host %q, port %d, db %q, user %q", got.Host, got.Port, got.Database, got.User)
	}
}
