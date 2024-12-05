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

package permission_api

import (
	"errors"
	"fmt"
	"github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"net/http"
	"strconv"
)

type PermissionApi struct {
	client client.Client
}

func NewPermissionApi(url string) *PermissionApi {
	return NewPermissionApiFromClient(client.New(url))
}

func NewPermissionApiFromClient(client client.Client) *PermissionApi {
	return &PermissionApi{client: client}
}

func (a PermissionApi) UserHasDevicesReadAccess(ids []string, authorization string) (result bool, err error) {
	response, err, code := a.client.CheckMultiplePermissions(authorization, "devices", ids, client.Read)
	if err != nil {
		return result, fmt.Errorf("permission API - could not check access rights: %w", err)
	}
	result = false
	if code != http.StatusOK {
		err = errors.New("permission API - could not check access rights: " + strconv.Itoa(code))
		return
	}
	if len(response) > 0 {
		for _, access := range response {
			if !access {
				return result, nil
			}
		}
		result = true
	}
	return result, nil
}
