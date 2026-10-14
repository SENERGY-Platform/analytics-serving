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
			util.Logger.Error("Validation error", "error", err.Param())
			field := fieldPath(err)
			switch err.Tag() {
			case "required":
				errors[field] = append(errors[field], "The field '"+field+"' is required")
				break
			case "min":
				errors[field] = append(errors[field], "The field '"+field+"' must be at least have "+err.Param()+" chars")
				break
			case "max":
				unit := "characters"
				if err.Kind() != reflect.String {
					unit = "elements"
				}
				errors[field] = append(errors[field], "The field '"+field+"' must not have more than "+err.Param()+" "+unit)
			case "oneof":
				errors[field] = append(errors[field], "The field '"+field+"' must be one of: "+strings.Join(strings.Fields(err.Param()), ", "))
			}
		}
		return false, errors
	}

	return true, nil
}

// fieldPath names a field by its JSON path below the validated struct, e.g. "Values[0].Name", so that a nested
// field does not collide with a top-level field of the same name.
func fieldPath(err validator.FieldError) string {
	if _, path, found := strings.Cut(err.Namespace(), "."); found {
		return path
	}
	return err.Field()
}
