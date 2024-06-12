/*
 * Copyright 2024 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
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
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"log"
	"strings"
)

func (f *Serving) GetExportDatabases(userId string, args map[string][]string) (databases []ExportDatabase, errs []error) {
	tx := DB.Select("*").Where("public = TRUE OR user_id = ?", userId)
	for arg, value := range args {
		if arg == "limit" {
			tx = tx.Limit(value[0])
		}
		if arg == "offset" {
			tx = tx.Offset(value[0])
		}
		if arg == "order" {
			order := strings.SplitN(value[0], ":", 2)
			tx = tx.Order(order[0] + " " + order[1])
		}
		if arg == "search" {
			search := strings.SplitN(value[0], ":", 2)
			if len(search) > 1 {
				allowed := []string{"name", "description", "type"}
				if StringInSlice(search[0], allowed) {
					tx = tx.Where(search[0]+" LIKE ?", "%"+search[1]+"%")
				}
			} else {
				tx = tx.Where("name LIKE ?", "%"+value[0]+"%")
			}
		}
		if arg == "deployment" {
			tx = tx.Where("`deployment` = ?", value[0])
		}
		if arg == "public" {
			if value[0] == "true" {
				tx = tx.Where("`public` = TRUE")
			} else {
				tx = tx.Where("`public` = FALSE")
			}
		}
		if arg == "owner" {
			if value[0] == "true" {
				tx = tx.Where("`user_id` = ?", userId)
			} else {
				tx = tx.Where("`user_id` != ?", userId)
			}
		}
	}
	errs = tx.Find(&databases).GetErrors()
	if len(errs) > 0 {
		for _, err := range errs {
			if gorm.IsRecordNotFoundError(err) {
				return databases, nil
			}
		}
		log.Println("listing export-databases failed - " + fmt.Sprint(errs))
		return
	}
	return
}

func (f *Serving) GetExportDatabase(id string, userId string) (database ExportDatabase, errs []error) {
	errs = DB.Where("id = ? AND (user_id = ? OR public = TRUE)", id, userId).First(&database).GetErrors()
	if len(errs) > 0 {
		log.Println("retrieving export-database failed - " + id + " - " + fmt.Sprint(errs))
		return
	}
	return
}

func (f *Serving) CreateExportDatabase(id string, req ExportDatabaseRequest, userId string) (database ExportDatabase, errs []error) {
	if id == "" {
		id = uuid.New().String()
		if f.exportDatabaseIdPrefix != "" {
			id = f.exportDatabaseIdPrefix + id
		}
	}
	database = populateExportDatabase(id, req, userId)
	if driver, ok := f.driver.(ExportWorkerKafkaApi); ok {
		err := driver.CreateFilterTopic(database.EwFilterTopic, true)
		if err != nil {
			errs = append(errs, err)
			return
		}
	}
	DB.NewRecord(database)
	errs = DB.Create(&database).GetErrors()
	if len(errs) > 0 {
		log.Println("creating export-database failed - " + fmt.Sprint(errs))
		return
	}
	log.Println("successfully created export-database - " + database.ID)
	return
}

func (f *Serving) UpdateExportDatabase(id string, req ExportDatabaseRequest, userId string) (database ExportDatabase, errs []error) {
	errs = DB.Where("id = ? AND user_id = ?", id, userId).First(&database).GetErrors()
	if len(errs) > 0 {
		for _, err := range errs {
			if gorm.IsRecordNotFoundError(err) {
				database, errs = f.CreateExportDatabase(id, req, userId)
				return
			}
		}
		log.Println("updating export-database failed - " + id + " - " + fmt.Sprint(errs))
		return
	}
	dbType := database.Type
	dbEwFilterTopic := database.EwFilterTopic
	database = populateExportDatabase(id, req, userId)
	if database.Type != dbType || database.EwFilterTopic != dbEwFilterTopic {
		errs = append(errs, errors.New("changing 'Type' or 'EwFilterTopic' not allowed"))
	} else {
		errs = DB.Save(&database).GetErrors()
	}
	if len(errs) > 0 {
		log.Println("updating export-database failed - " + id + " - " + fmt.Sprint(errs))
		return
	}
	log.Println("successfully updated export-database - " + database.ID)
	return
}

func (f *Serving) DeleteExportDatabase(id string, userId string) (errs []error) {
	var database ExportDatabase
	errs = DB.Where("id = ? AND user_id = ?", id, userId).First(&database).GetErrors()
	if len(errs) > 0 {
		log.Println("deleting export-database failed - " + id + " - " + fmt.Sprint(errs))
		return
	}
	errs = DB.Delete(&database).GetErrors()
	if len(errs) > 0 {
		log.Println("deleting export-database failed - " + id + " - " + fmt.Sprint(errs))
	}
	return
}

func populateExportDatabase(id string, req ExportDatabaseRequest, userId string) (database ExportDatabase) {
	database = ExportDatabase{
		ID:            id,
		Name:          req.Name,
		Description:   req.Description,
		Type:          req.Type,
		Deployment:    req.Deployment,
		Url:           req.Url,
		EwFilterTopic: req.EwFilterTopic,
		Public:        req.Public,
		UserId:        userId,
	}
	return
}
