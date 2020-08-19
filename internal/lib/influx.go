/*
 * Copyright 2020 InfAI (CC SES)
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

package lib

import (
	"fmt"
	influxClient "github.com/influxdata/influxdb1-client/v2"
)

type Influx struct {
	client influxClient.Client
}

func NewInflux() *Influx {
	client, err := influxClient.NewHTTPClient(influxClient.HTTPConfig{
		Addr:     "http://" + GetEnv("INFLUX_DB_HOST", "") + ":" + GetEnv("INFLUX_DB_PORT", "8086"),
		Username: GetEnv("INFLUX_DB_USERNAME", "root"),
		Password: GetEnv("INFLUX_DB_PASSWORD", ""),
	})
	if err != nil {
		fmt.Println(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	return &Influx{client}
}

func (i *Influx) DropMeasurement(instance Instance) (errors []error) {
	q := influxClient.NewQuery("DROP MEASUREMENT "+"\""+instance.Measurement+"\"", instance.Database, "")
	response, err := i.client.Query(q)
	if err != nil {
		errors = append(errors, err)
	}
	if response.Error() != nil {
		errors = append(errors, response.Error())
	}
	return
}
