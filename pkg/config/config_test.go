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
	t.Setenv("MYSQL_PW", "mysql-secret-from-env")
	t.Setenv("INFLUX_DB_PASSWORD", "influx-secret-from-env")
	cfg, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.Password.Value() != "mysql-secret-from-env" || cfg.InfluxConfig.Password.Value() != "influx-secret-from-env" {
		t.Fatalf("passwords not loaded from env: mysql %q, influx %q", cfg.MySQL.Password.Value(), cfg.InfluxConfig.Password.Value())
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"mysql-secret-from-env", "influx-secret-from-env"} {
		if strings.Contains(string(b), secret) {
			t.Errorf("config JSON contains %q: %s", secret, b)
		}
	}
}

func TestConfigFileSetsPasswords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"mysql": {"password": "mysql-secret-from-file"}, "influx_config": {"password": "influx-secret-from-file"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.Password.Value() != "mysql-secret-from-file" || cfg.InfluxConfig.Password.Value() != "influx-secret-from-file" {
		t.Fatalf("passwords not loaded from file: mysql %q, influx %q", cfg.MySQL.Password.Value(), cfg.InfluxConfig.Password.Value())
	}
}

// Parallel deletes each take a connection, so the pool needs a ceiling by default.
func TestConfigLimitsTheMySQLPool(t *testing.T) {
	cfg, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.MaxOpenConns != 25 || cfg.MySQL.MaxIdleConns != 10 {
		t.Fatalf("default pool limits: open %d, idle %d, want 25 and 10", cfg.MySQL.MaxOpenConns, cfg.MySQL.MaxIdleConns)
	}

	t.Setenv("MYSQL_MAX_OPEN_CONNS", "40")
	t.Setenv("MYSQL_MAX_IDLE_CONNS", "5")
	cfg, err = New("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.MaxOpenConns != 40 || cfg.MySQL.MaxIdleConns != 5 {
		t.Fatalf("pool limits from env: open %d, idle %d, want 40 and 5", cfg.MySQL.MaxOpenConns, cfg.MySQL.MaxIdleConns)
	}
}
