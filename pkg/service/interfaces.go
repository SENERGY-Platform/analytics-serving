/*
 * Copyright 2025 InfAI (CC SES)
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

import "github.com/SENERGY-Platform/analytics-serving/lib"

type Driver interface {
	CreateInstance(instance *lib.Instance, dataFields string, tagFields string) (serviceId string, err error)
	DeleteInstance(instance *lib.Instance) (err error)
}

type PipelineApiService interface {
	UserHasPipelineAccess(id string, authorization string) (bool, error)
}

type ImportDeployService interface {
	UserHasImportAccess(id string, authorization string) (bool, error)
}

type ExportWorkerKafkaApi interface {
	CreateFilterTopic(topic string, checkExists bool) error
	InitFilterTopics(serving *Serving) error
}
