/*
 * Copyright 2024 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mocks

import (
	"github.com/SENERGY-Platform/analytics-serving/internal/lib"
	"github.com/google/uuid"
)

type Driver struct{}

func (this Driver) CreateInstance(instance *lib.Instance, dataFields string, tagFields string) (serviceId string, err error) {
	return uuid.NewString(), nil
}

func (this Driver) DeleteInstance(instance *lib.Instance) (err error) {
	return nil
}
