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

package mocks

import (
	"fmt"
	"sync"
	"time"

	"github.com/SENERGY-Platform/analytics-serving/lib"
	"github.com/google/uuid"
)

// RecordingDriver records successful DeleteInstance calls and the highest number of calls in flight at once.
// Delay, Delays and FailIds must not be changed while the driver is in use.
type RecordingDriver struct {
	Delay   time.Duration            // applied to every DeleteInstance call without an entry in Delays
	Delays  map[string]time.Duration // per instance id
	FailIds map[string]bool          // DeleteInstance returns an error for these instance ids

	mux         sync.Mutex
	inFlight    int
	maxInFlight int
	deleted     []string
}

func (this *RecordingDriver) CreateInstance(instance *lib.Instance, dataFields string, tagFields string) (serviceId string, err error) {
	return uuid.NewString(), nil
}

func (this *RecordingDriver) DeleteInstance(instance *lib.Instance) (err error) {
	id := instance.ID.String()
	this.mux.Lock()
	this.inFlight++
	this.maxInFlight = max(this.maxInFlight, this.inFlight)
	this.mux.Unlock()

	delay, ok := this.Delays[id]
	if !ok {
		delay = this.Delay
	}
	time.Sleep(delay)

	this.mux.Lock()
	defer this.mux.Unlock()
	this.inFlight--
	if this.FailIds[id] {
		return fmt.Errorf("driver failure for %v", id)
	}
	this.deleted = append(this.deleted, id)
	return nil
}

// MaxInFlight returns the highest number of concurrent DeleteInstance calls seen so far.
func (this *RecordingDriver) MaxInFlight() int {
	this.mux.Lock()
	defer this.mux.Unlock()
	return this.maxInFlight
}

// Deleted returns the ids of successful DeleteInstance calls in completion order, one entry per call.
func (this *RecordingDriver) Deleted() []string {
	this.mux.Lock()
	defer this.mux.Unlock()
	return append([]string(nil), this.deleted...)
}
