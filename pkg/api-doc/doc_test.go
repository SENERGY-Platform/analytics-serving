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

package api_doc

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
)

func TestPublishAsyncapiDocClosesFileOnce(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	const doc = `{"asyncapi":"3.0.0"}`
	if err := os.WriteFile(filepath.Join(dir, "docs", "asyncapi.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var logs bytes.Buffer
	oldLogger := util.Logger
	util.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
	defer func() { util.Logger = oldLogger }()

	PublishAsyncapiDoc(srv.URL)

	if received != doc {
		t.Errorf("unexpected body %q", received)
	}
	if strings.Contains(logs.String(), "error") {
		t.Errorf("unexpected error log: %s", logs.String())
	}
}
