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

package import_deploy_api

import (
	"errors"
	"strconv"

	"github.com/parnurzeal/gorequest"
)

type ImportDeployApi struct {
	url string
}

func NewImportDeployApi(url string) *ImportDeployApi {
	return &ImportDeployApi{url}
}
func (i *ImportDeployApi) UserHasImportAccess(id string, authorization string) (hasAccess bool, err error) {
	hasAccess = false
	request := gorequest.New()
	request.Get(i.url+"/instances/"+id).Set("Authorization", authorization)
	resp, body, e := request.End()

	if resp.StatusCode != 200 {
		err = errors.New("import deploy API - could not get import: " + strconv.Itoa(resp.StatusCode) + " " + body)
		return
	}
	if len(e) > 0 {
		err = errors.New("import deploy API - could not get import: an error occurred")
		return
	}
	if resp.StatusCode == 200 {
		hasAccess = true
		return
	}
	return
}
