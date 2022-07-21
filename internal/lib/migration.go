/*
 * Copyright 2019 InfAI (CC SES)
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
	"log"

	"github.com/jinzhu/gorm"
)

type Migration struct {
	db *gorm.DB
}

func NewMigration(db *gorm.DB) *Migration {
	return &Migration{db}
}

func (m *Migration) Migrate() {
	if !DB.HasTable("instances") {
		log.Println("Creating instances table.")
		DB.CreateTable(&Instance{})
	}
	DB.AutoMigrate(&Instance{})
	DB.Model(&Instance{}).AddForeignKey("export_database_id", "export_databases(id)", "RESTRICT", "CASCADE")
	if !DB.HasTable("values") {
		log.Println("Creating values table.")
		DB.CreateTable(&Value{})
	}
	DB.AutoMigrate(&Value{})
	DB.Model(&Value{}).AddForeignKey("instance_id", "instances(id)", "CASCADE", "CASCADE")
	if !DB.HasTable("export_databases") {
		log.Println("Creating export_databases table.")
		DB.CreateTable(&ExportDatabase{})
	}
	DB.AutoMigrate(&ExportDatabase{})
}
