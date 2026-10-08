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

package db

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SENERGY-Platform/analytics-serving/pkg/config"
	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

var DB *gorm.DB

func Init(config *config.PostgresConfig) (err error) {
	address := config.Host + ":" + strconv.Itoa(config.Port) + "/" + config.Database
	util.Logger.Info(fmt.Sprintf("connecting to %s as %s", address, config.User))
	db, err := gorm.Open("postgres", connectionString(config))
	if err != nil {
		return
	} else {
		util.Logger.Info("connected to DB")
	}
	db.DB().SetMaxOpenConns(config.MaxOpenConns)
	db.DB().SetMaxIdleConns(config.MaxIdleConns)
	db.LogMode(false)
	DB = db
	return
}

// connectionString quotes every value, since an unquoted password or name with a space, quote or backslash
// would be cut off or unescaped by the key=value parser of lib/pq.
func connectionString(config *config.PostgresConfig) string {
	quote := func(value string) string {
		return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(value) + "'"
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		quote(config.Host), config.Port, quote(config.User), quote(config.Password.Value()), quote(config.Database))
}

func Close() (err error) {
	err = DB.Close()
	return
}

func GetDB() *gorm.DB {
	return DB
}
