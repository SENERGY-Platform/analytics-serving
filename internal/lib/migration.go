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
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"log"
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

type ExportDatabaseBase struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	External    bool   `json:"external"`
	Url         string `json:"url"`
	Public      bool   `json:"public"`
	UserID      string `json:"user_id"`
}

func (m *Migration) TmpMigrate() (err error) {
	topicMap := map[string]string{}
	err = json.Unmarshal([]byte(GetEnv("EW_FILTER_TOPIC_MAP", "")), &topicMap)
	if err != nil {
		return
	}
	migrationMap := map[string]ExportDatabaseBase{}
	err = json.Unmarshal([]byte(GetEnv("MIGRATION_MAP", "")), &migrationMap)
	if err != nil {
		return
	}
	idPrefix := GetEnv("EXPORT_DATABASE_ID_PREFIX", "")
	log.Println(idPrefix)
	for key, val := range topicMap {
		var errs []error
		var instances []Instance
		DB.Where("database_type = ? AND (export_database_id IS null OR export_database_id = ?)", key, "").Find(&instances).GetErrors()
		if len(errs) > 0 {
			err = errors.New(fmt.Sprint(errs))
			return
		}
		if len(instances) > 0 {
			log.Println(fmt.Sprintf("found %d exports for '%s'", len(instances), key))
			id := uuid.New().String()
			if idPrefix != "" {
				id = idPrefix + id
			}
			database := ExportDatabase{
				ID:            id,
				Name:          migrationMap[key].Name,
				Description:   migrationMap[key].Description,
				Type:          key,
				External:      migrationMap[key].External,
				Url:           migrationMap[key].Url,
				EwFilterTopic: val,
				UserId:        migrationMap[key].UserID,
				Public:        migrationMap[key].Public,
			}
			log.Println(fmt.Sprintf("generated ExportDatabase: %+v", database))
			DB.NewRecord(database)
			errs = DB.Create(&database).GetErrors()
			if len(errs) > 0 {
				err = errors.New(fmt.Sprint(errs))
				return
			}
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
