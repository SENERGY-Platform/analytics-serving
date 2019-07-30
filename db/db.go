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

package db

import (
	"analytics-serving/lib"
	"fmt"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

var DB *gorm.DB

func Init() {
	connectionString := lib.GetEnv("MYSQL_USER", "") + ":" +
		lib.GetEnv("MYSQL_PW", "") +
		"@(" + lib.GetEnv("MYSQL_HOST", "") + ":3306)" +
		"/" + lib.GetEnv("MYSQL_DB", "") +
		"?charset=utf8&parseTime=True&loc=Local"
	fmt.Println("Connecting to: " + connectionString)
	db, err := gorm.Open("mysql", connectionString)
	if err != nil {
		panic("failed to connect database: " + err.Error())
	} else {
		fmt.Println("Connected to DB.")
	}
	db.Set("gorm:table_options", "ENGINE=InnoDB")
	db.LogMode(false)
	DB = db
}

func Close() {
	DB.Close()
}

func GetDB() *gorm.DB {
	return DB
}
