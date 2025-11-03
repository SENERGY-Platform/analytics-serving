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

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/SENERGY-Platform/analytics-serving/lib"
	"github.com/SENERGY-Platform/analytics-serving/pkg/config"
	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	influxClient "github.com/influxdata/influxdb1-client/v2"
)

type Influx interface {
	ForceDeleteMeasurement(id string, userId string, instance lib.Instance) (errs []error)
}

type InfluxImpl struct {
	client influxClient.Client
}

func NewInflux(cfg config.InfluxConfig) *InfluxImpl {
	client, err := influxClient.NewHTTPClient(influxClient.HTTPConfig{
		Addr:     cfg.Protocol + "://" + cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Username: cfg.User,
		Password: cfg.Password,
	})
	if err != nil {
		util.Logger.Error("could not connect to influx", "error", err)
	}
	defer func() {
		if err = client.Close(); err != nil {
			util.Logger.Error("could not close influx connection", "error", err)
		}
	}()
	return &InfluxImpl{client}
}

func (i *InfluxImpl) ForceDeleteMeasurement(id string, userId string, instance lib.Instance) (errs []error) {
	defer func() {
		if err := recover(); err != nil {
			errs = append(errs, errors.New("force delete influx measurement failed - panic occurred: "+fmt.Sprint(err)))
		}
	}()
	for {
		errs = i.dropMeasurement(instance)
		if len(errs) > 0 {
			return
		}
		measurements, err := i.getMeasurements(userId)
		if err != nil {
			errs = append(errs, err)
			return
		}
		if !util.StringInSlice(id, measurements) {
			break
		}
	}
	return
}

func (i *InfluxImpl) dropMeasurement(instance lib.Instance) (errors []error) {
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

func (i *InfluxImpl) getMeasurements(userId string) (measurements []string, err error) {
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
