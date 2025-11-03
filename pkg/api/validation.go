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

package api

import (
	"log"
	"reflect"
	"strings"

	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	"github.com/go-playground/validator/v10"
)

func ValidateInputs(dataSet interface{}) (bool, map[string][]string) {
	var validate *validator.Validate

	validate = validator.New()

	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	err := validate.Struct(dataSet)

	if err != nil {

		errors := make(map[string][]string)

		if _, ok := err.(*validator.InvalidValidationError); ok {
			util.Logger.Error("Invalid validation error", "error", err)
			return false, errors
		}

		for _, err := range err.(validator.ValidationErrors) {
			log.Println(err.Param())
			switch err.Tag() {
			case "required":
				errors[err.Field()] = append(errors[err.Field()], "The field '"+err.Field()+"' is required")
				break
			case "min":
				errors[err.Field()] = append(errors[err.Field()], "The field '"+err.Field()+"' must be at least have "+err.Param()+" chars")
				break
			}
		}
		return false, errors
	}

	return true, nil
}
