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
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
)

type Migration struct {
	db            *gorm.DB
	migrationInfo string
}

func NewMigration(db *gorm.DB, migrationInfo string) *Migration {
	return &Migration{db: db, migrationInfo: migrationInfo}
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

type MigrationInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Type          string `json:"type"`
	Deployment    string `json:"deployment"`
	Url           string `json:"url"`
	EwFilterTopic string `json:"ew_filter_topic"`
	UserID        string `json:"user_id"`
	Public        bool   `json:"public"`
}

func (m *Migration) TmpMigrate() (err error) {
	if m.migrationInfo != "" {
		log.Println("RUNNING TEMPORARY MIGRATION!")
		var migrationInfo MigrationInfo
		err = json.Unmarshal([]byte(m.migrationInfo), &migrationInfo)
		if err != nil {
			return
		}
		var database ExportDatabase
		errs := DB.Where("id = ?", migrationInfo.ID).First(&database).GetErrors()
		if len(errs) > 0 {
			err = errors.New(fmt.Sprint(errs))
			for _, e := range errs {
				if gorm.IsRecordNotFoundError(e) {
					err = nil
					database = ExportDatabase{
						ID:            migrationInfo.ID,
						Name:          migrationInfo.Name,
						Description:   migrationInfo.Description,
						Type:          migrationInfo.Type,
						Deployment:    migrationInfo.Deployment,
						Url:           migrationInfo.Url,
						EwFilterTopic: migrationInfo.EwFilterTopic,
						UserId:        migrationInfo.UserID,
						Public:        migrationInfo.Public,
					}
					log.Println(fmt.Sprintf("generated ExportDatabase: %+v", database))
					DB.NewRecord(database)
					errs2 := DB.Create(&database).GetErrors()
					if len(errs2) > 0 {
						err = errors.New(fmt.Sprint(errs2))
						return
					}
					break
				}
			}
			if err != nil {
				return
			}
		} else {
			log.Println("export-database with ID '" + database.ID + "' already exists")
		}
		var instances []Instance
		errs = DB.Where("export_database_id IS null OR export_database_id = ?", "").Find(&instances).GetErrors()
		if len(errs) > 0 {
			err = errors.New(fmt.Sprint(errs))
			return
		}
		if len(instances) > 0 {
			log.Println(fmt.Sprintf("found %d exports", len(instances)))
			for _, instance := range instances {
				log.Println("migrating export '" + instance.ID.String() + "'")
				instance.ExportDatabaseID = database.ID
				instance.ExportDatabase = database
				errs = DB.Save(&instance).GetErrors()
				if len(errs) > 0 {
					err = errors.New(fmt.Sprint(errs))
					return
				}
			}
		}
	}
	return
}
