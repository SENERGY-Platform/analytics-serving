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
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// runWithTimeout fails the test instead of hanging when forEachParallel deadlocks.
func runWithTimeout(t *testing.T, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("forEachParallel did not return")
	}
}

func TestForEachParallelCallsEveryIndexExactlyOnce(t *testing.T) {
	const n = 50
	var calls [n]atomic.Int32
	runWithTimeout(t, func() {
		forEachParallel(n, 4, func(i int) {
			calls[i].Add(1)
		})
	})
	for i := range calls {
		if got := calls[i].Load(); got != 1 {
			t.Errorf("index %d called %d times, want 1", i, got)
		}
	}
}

func TestForEachParallelRunsUpToLimitCallsAtOnce(t *testing.T) {
	const n = 20
	const limit = 4
	var inFlight, maxInFlight atomic.Int32
	runWithTimeout(t, func() {
		forEachParallel(n, limit, func(i int) {
			current := inFlight.Add(1)
			for {
				seen := maxInFlight.Load()
				if current <= seen || maxInFlight.CompareAndSwap(seen, current) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			inFlight.Add(-1)
		})
	})
	if got := maxInFlight.Load(); got != limit {
		t.Errorf("max calls in flight = %d, want %d", got, limit)
	}
}

func TestForEachParallelTreatsNonPositiveLimitAsSerial(t *testing.T) {
	for _, limit := range []int{0, -1} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			var calls atomic.Int32
			runWithTimeout(t, func() {
				forEachParallel(3, limit, func(int) { calls.Add(1) })
			})
			if got := calls.Load(); got != 3 {
				t.Errorf("calls = %d, want 3", got)
			}
		})
	}
}

func TestForEachParallelReturnsImmediatelyForZeroItems(t *testing.T) {
	runWithTimeout(t, func() {
		forEachParallel(0, maxParallelDeletes, func(int) { t.Error("fn called for n = 0") })
	})
}

func TestForEachParallelReraisesPanicAfterAllOtherCallsFinished(t *testing.T) {
	const n = 6
	var finished atomic.Int32
	var recovered any
	runWithTimeout(t, func() {
		defer func() { recovered = recover() }()
		forEachParallel(n, 2, func(i int) {
			if i == 1 {
				panic("boom")
			}
			time.Sleep(20 * time.Millisecond)
			finished.Add(1)
		})
	})
	msg, ok := recovered.(string)
	if !ok || !strings.Contains(msg, "boom") {
		t.Fatalf("recovered %v, want the panic value of fn", recovered)
	}
	if got := finished.Load(); got != n-1 {
		t.Errorf("finished calls when panic surfaced = %d, want %d", got, n-1)
	}
}

func TestForEachParallelWritesToDistinctSlotsWithoutRace(t *testing.T) {
	const n = 100
	results := make([]int, n)
	runWithTimeout(t, func() {
		forEachParallel(n, maxParallelDeletes, func(i int) { results[i] = i * i })
	})
	for i, v := range results {
		if v != i*i {
			t.Fatalf("results[%d] = %d, want %d", i, v, i*i)
		}
	}
}

func TestUniqueInOrderKeepsFirstOccurrence(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{in: nil, want: []string{}},
		{in: []string{}, want: []string{}},
		{in: []string{"a"}, want: []string{"a"}},
		{in: []string{"a", "b", "a", "c", "b"}, want: []string{"a", "b", "c"}},
		{in: []string{"c", "c", "c"}, want: []string{"c"}},
		{in: []string{"", "a", ""}, want: []string{"", "a"}},
	}
	for _, c := range cases {
		if got := uniqueInOrder(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("uniqueInOrder(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
