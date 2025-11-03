/*
 * Copyright 2018 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pipeline_api

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/SENERGY-Platform/analytics-pipeline/lib"
	"github.com/parnurzeal/gorequest"
)

type PipelineApi struct {
	url string
}

func NewPipelineApi(url string) *PipelineApi {
	return &PipelineApi{url}
}
func (p *PipelineApi) UserHasPipelineAccess(id string, authorization string) (hasAccess bool, err error) {
	hasAccess = false
	request := gorequest.New()
	request.Get(p.url+"/pipeline/"+id).Set("Authorization", authorization)
	resp, body, e := request.End()

	if resp.StatusCode != 200 {
		err = errors.New("pipeline API - could not get pipeline from pipeline registry: " + strconv.Itoa(resp.StatusCode) + " " + body)
	}
	if len(e) > 0 {
		err = errors.New("pipeline API - could not get pipeline from pipeline registry: an error occurred")
		return
	}
	pipe := lib.Pipeline{}
	err = json.Unmarshal([]byte(body), &pipe)
	if err != nil {
		err = errors.New("pipeline API  - could not parse pipeline: " + err.Error())
		return
	}
	if pipe.Id == id {
		hasAccess = true
	}
	return
}
