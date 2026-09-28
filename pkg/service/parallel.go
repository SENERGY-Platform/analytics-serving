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
	"runtime/debug"
	"sync"
)

type recoveredPanic struct {
	value any
	stack []byte
}

// forEachParallel calls fn once for every index in [0, n) with at most limit calls in flight and returns when all
// calls have finished. A panic in fn is re-raised on the calling goroutine afterwards, so it reaches the caller's
// recovery instead of terminating the process.
func forEachParallel(n int, limit int, fn func(i int)) {
	if limit < 1 {
		limit = 1
	}
	panics := make([]*recoveredPanic, n)
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := range n {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					panics[i] = &recoveredPanic{value: r, stack: debug.Stack()}
				}
				<-sem
			}()
			fn(i)
		})
	}
	wg.Wait()
	for i, p := range panics {
		if p != nil {
			panic(fmt.Sprintf("parallel call %d panicked: %v\n\n%s", i, p.value, p.stack))
		}
	}
}

// uniqueInOrder returns ids without repetitions, keeping the first occurrence of each.
func uniqueInOrder(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
