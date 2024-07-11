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
	"net/http"
	"strconv"
)

type ListOptions struct {
	Limit            int
	Offset           int
	Asc              bool
	OrderBy          string
	SearchBy         string // Allowed values: "name", "description", "entity_name", "service_name"
	SearchValue      string
	Generated        *bool
	ExportDatabaseID string
	InternalOnly     *bool
}

func (l *ListOptions) toQuery() (query string) {
	query = "?"
	if l.Limit != 0 {
		query += "limit=" + strconv.Itoa(l.Limit) + "&"
	}

	if l.Offset != 0 {
		query += "offset=" + strconv.Itoa(l.Offset) + "&"
	}

	if l.OrderBy != "" {
		query += "order=" + l.OrderBy + ":"
		if l.Asc {
			query += "asc"
		} else {
			query += "desc"
		}
		query += "&"
	}

	if l.SearchBy != "" && l.SearchValue != "" {
		query += "search=" + l.SearchBy + ":" + l.SearchValue + "&"
	}

	if l.Generated != nil {
		query += "generated="
		if *l.Generated {
			query += "true"
		} else {
			query += "false"
		}
		query += "&"
	}

	if l.ExportDatabaseID != "" {
		query += "export_database_id=" + l.ExportDatabaseID + "&"
	}

	if l.InternalOnly != nil {
		query += "internal_only="
		if *l.InternalOnly {
			query += "true"
		} else {
			query += "false"
		}
		query += "&"
	}

	return query[:len(query)-1]
}

func (c *Client) ListInstances(token string, options *ListOptions) (result InstancesResponse, err error) {
	url := c.baseUrl + "/instance"
	if options != nil {
		url += options.toQuery()
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return result, err
	}

	req.Header.Add("Authorization", token)

	return do[InstancesResponse](req)
}

func (c *Client) ListInstancesAsAdmin(token string, options *ListOptions) (result Instances, err error) {
	url := c.baseUrl + "/admin/instance"
	if options != nil {
		url += options.toQuery()
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return result, err
	}

	req.Header.Add("Authorization", token)

	return do[Instances](req)
}
