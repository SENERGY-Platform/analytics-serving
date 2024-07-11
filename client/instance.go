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

package client

import (
	"errors"
	"net/http"
)

func (c *Client) GetInstance(token string, id string) (result Instance, err error) {
	if id == "" {
		return result, errors.New("malformed id")
	}
	req, err := http.NewRequest(http.MethodGet, c.baseUrl+"/instance/"+id, nil)
	if err != nil {
		return result, err
	}

	req.Header.Add("Authorization", token)

	return do[Instance](req)
}
