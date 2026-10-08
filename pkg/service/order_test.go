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

package service

import (
	"errors"
	"slices"
	"testing"
)

// Rejected for both tables. The functions return before touching the database, which is nil here.
var illegalOrders = []string{
	"",
	"name",
	"name:",
	":asc",
	"name:up",
	"name:asc ",
	"name :asc",
	"Name:asc",
	"name:asc:desc",
	"name:asc,id",
	"name:asc;DROP TABLE x",
	"name;DROP TABLE x:asc",
	"name:aſc", // folds to "asc" under Unicode case folding, but not under ToLower
	"values:asc",
}

func TestGetInstancesOrder(t *testing.T) {
	legal := map[string]order{
		"name:asc":        {"name", "asc"},
		"name:DESC":       {"name", "desc"},
		"created_at:Desc": {"created_at", "desc"},
		"offset:asc":      {"offset", "asc"},
	}
	for value, want := range legal {
		got, err := parseOrder(value, instanceOrderColumns)
		if err != nil || got != want {
			t.Errorf("parseOrder(%q) = %+v, %v; want %+v", value, got, err, want)
		}
	}

	f := &Serving{}
	for _, value := range append(illegalOrders, "url:asc") {
		assertQueryError(t, value, func() []error {
			_, _, errs := f.GetInstances("user", map[string][]string{"order": {value}}, true, "")
			return errs
		})
	}
}

func TestGetExportDatabasesOrder(t *testing.T) {
	legal := map[string]order{
		"name:asc":    {"name", "asc"},
		"type:DESC":   {"type", "desc"},
		"public:desc": {"public", "desc"},
	}
	for value, want := range legal {
		got, err := parseOrder(value, exportDatabaseOrderColumns)
		if err != nil || got != want {
			t.Errorf("parseOrder(%q) = %+v, %v; want %+v", value, got, err, want)
		}
	}

	f := &Serving{}
	for _, value := range append(illegalOrders, "entity_name:asc") {
		assertQueryError(t, value, func() []error {
			_, errs := f.GetExportDatabases("user", map[string][]string{"order": {value}})
			return errs
		})
	}
}

func assertQueryError(t *testing.T, value string, list func() []error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("order %q was not rejected and reached the database: %v", value, r)
		}
	}()
	errs := list()
	var queryErr *QueryError
	if len(errs) != 1 || !errors.As(errs[0], &queryErr) || queryErr.Parameter != "order" {
		t.Errorf("order %q: got %v, want one QueryError for order", value, errs)
	}
}

func TestPageOrder(t *testing.T) {
	id := order{"id", "asc"}
	filterType := order{"filter_type", "desc"}
	cases := []struct {
		name     string
		args     map[string][]string
		o        order
		hasOrder bool
		want     []order
	}{
		{"no order, not paged", map[string][]string{}, order{}, false, nil},
		{"order, not paged", map[string][]string{"order": {"filter_type:desc"}}, filterType, true, []order{filterType}},
		{"order and limit", map[string][]string{"order": {"filter_type:desc"}, "limit": {"10"}}, filterType, true, []order{filterType, id}},
		{"order and offset", map[string][]string{"order": {"filter_type:desc"}, "offset": {"10"}}, filterType, true, []order{filterType, id}},
		{"limit without order", map[string][]string{"limit": {"10"}}, order{}, false, []order{id}},
		{"offset without order", map[string][]string{"offset": {"10"}}, order{}, false, []order{id}},
		{"paged by id already", map[string][]string{"limit": {"10"}}, order{"id", "desc"}, true, []order{{"id", "desc"}}},
	}
	for _, c := range cases {
		if got := pageOrder(c.args, c.o, c.hasOrder); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
