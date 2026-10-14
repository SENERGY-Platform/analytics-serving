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

package api

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/SENERGY-Platform/analytics-serving/pkg/service"
)

func TestCollectInputErrors(t *testing.T) {
	timePath := &service.InputError{Field: "TimePath", Message: "a"}
	format := &service.InputError{Field: "TimestampFormat", Message: "b"}
	cases := []struct {
		name string
		err  error
		want map[string][]string
	}{
		{"nil", nil, map[string][]string{}},
		{"other error", errors.New("db down"), map[string][]string{}},
		{"denied source", fmt.Errorf("x: %w", service.ErrSourceAccessDenied), map[string][]string{}},
		{"single", timePath, map[string][]string{"TimePath": {"a"}}},
		{"joined", errors.Join(timePath, format), map[string][]string{"TimePath": {"a"}, "TimestampFormat": {"b"}}},
		{"joined below a wrap", fmt.Errorf("create: %w", errors.Join(timePath, errors.New("other"), format)),
			map[string][]string{"TimePath": {"a"}, "TimestampFormat": {"b"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := map[string][]string{}
			collectInputErrors(tc.err, got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
