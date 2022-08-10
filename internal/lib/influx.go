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
	"errors"
	"fmt"
	"log"

	influxClient "github.com/influxdata/influxdb1-client/v2"
)

type Influx struct {
	client influxClient.Client
}

func NewInflux() *Influx {
	client, err := influxClient.NewHTTPClient(influxClient.HTTPConfig{
		Addr:     GetEnv("INFLUX_DB_PROTO", "http") + "://" + GetEnv("INFLUX_DB_HOST", "") + ":" + GetEnv("INFLUX_DB_PORT", "8086"),
		Username: GetEnv("INFLUX_DB_USERNAME", "root"),
		Password: GetEnv("INFLUX_DB_PASSWORD", ""),
	})
	if err != nil {
		log.Println("could not connect to influx: " + err.Error())
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Println(err)
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

func (i *Influx) GetMeasurements(userId string) (measurements []string, err error) {
	q := influxClient.NewQuery("SHOW MEASUREMENTS", userId, "")
	response, err := i.client.Query(q)
	if err != nil {
		return
	}
	if response.Error() != nil {
		err = response.Error()
		return
	}

	if len(response.Results[0].Series) > 0 {
		for _, measurement := range response.Results[0].Series[0].Values {
			measurements = append(measurements, measurement[0].(string))
		}
	}
	return measurements, err
}

func (i *Influx) forceDeleteMeasurement(id string, userId string, instance Instance) (errs []error) {
	defer func() {
		if err := recover(); err != nil {
			errs = append(errs, errors.New("force delete measurement failed - panic occurred: "+fmt.Sprint(err)))
		}
	}()
	for {
		errs = i.DropMeasurement(instance)
		if len(errs) > 0 {
			return
		}
		measurements, err := i.GetMeasurements(userId)
		if err != nil {
			errs = append(errs, err)
			return
		}
		if !StringInSlice(id, measurements) {
			break
		}
	}
	return
}
