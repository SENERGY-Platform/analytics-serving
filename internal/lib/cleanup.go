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
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	permV2Model "github.com/SENERGY-Platform/permissions-v2/pkg/model"
	"log"
	"slices"
	"time"
)

func (f *Serving) ExportInstanceCleanup(recheckWait time.Duration) error {
	if f.permissionsV2 != nil {
		log.Println("start exporting instance permissions cleanup")
		missingInPerm, missingInDb, err := f.findInconsistentExportInstanceIds()
		if err != nil {
			return err
		}

		allIds := []string{}
		allIds = append(allIds, missingInPerm...)
		allIds = append(allIds, missingInDb...)

		if len(missingInPerm) > 0 || len(missingInDb) > 0 {
			log.Printf("wait %v before rechecking and deleting of %v ids", recheckWait.String(), allIds)
			time.Sleep(recheckWait)
		}

		permIdsMap, err, _ := f.permissionsV2.CheckMultiplePermissions(permV2Client.InternalAdminToken, ExportInstancePermissionsTopic, allIds)
		if err != nil {
			return err
		}

		for _, id := range missingInDb {
			consistent, err := f.checkPermConsistency(permIdsMap, id)
			if err != nil {
				return err
			}
			if !consistent {
				log.Printf("inconsistent export instance found, remove %v from permissions\n", id)
				err, _ = f.permissionsV2.RemoveResource(permV2Client.InternalAdminToken, ExportInstancePermissionsTopic, id)
				if err != nil {
					return err
				}
			}
		}
		for _, id := range missingInPerm {
			consistent, err := f.checkPermConsistency(nil, id)
			if err != nil {
				return err
			}
			if !consistent {
				log.Printf("inconsistent export instance found, remove %v from local db\n", id)
				_, errs := f.DeleteInstance(id, "", true, permV2Client.InternalAdminToken)
				err = errors.Join(errs...)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (f *Serving) findInconsistentExportInstanceIds() (missingInPerm []string, missingInDb []string, err error) {
	if f.permissionsV2 != nil {
		done := false
		option := permV2Model.ListOptions{Limit: 100}
		knownPermIds := map[string]bool{}

		//loop permission ids
		for !done {
			done, err = func() (bool, error) {
				f.permMux.Lock()
				defer f.permMux.Unlock()

				//find ids in permissions
				ids, err, _ := f.permissionsV2.AdminListResourceIds(permV2Client.InternalAdminToken, ExportInstancePermissionsTopic, option)
				if err != nil {
					return true, err
				}
				option.Offset = option.Offset + option.Limit
				for _, id := range ids {
					knownPermIds[id] = true
				}

				//check if permission ids are in local db
				rows, err := DB.Model(&Instance{}).Select("id").Where("id IN ?", ids).Rows()
				if err != nil {
					return true, err
				}
				for rows.Next() {
					var id string
					err = rows.Scan(&id)
					if err != nil {
						return true, err
					}
					if !slices.Contains(ids, id) {
						missingInDb = append(missingInDb, id)
					}
				}
				err = rows.Err()
				if err != nil {
					return true, err
				}
				if int64(len(ids)) < option.Limit {
					return true, nil
				}
				return false, nil
			}()
			if err != nil {
				return nil, nil, err
			}
		}

		//loop db ids
		done = false
		option.Offset = 0
		for !done {
			done, err = func() (bool, error) {
				f.permMux.Lock()
				defer f.permMux.Unlock()
				rows, err := DB.Model(&Instance{}).Select("id").Limit(option.Limit).Offset(option.Offset).Rows()
				if err != nil {
					return true, err
				}
				var count int64 = 0
				for rows.Next() {
					var id string
					err = rows.Scan(&id)
					if err != nil {
						return true, err
					}
					if !knownPermIds[id] {
						missingInPerm = append(missingInPerm, id)
					}
					count++
				}
				err = rows.Err()
				if err != nil {
					return true, err
				}
				if count < option.Limit {
					return true, nil
				}
				option.Offset = option.Offset + option.Limit
				return false, nil
			}()
			if err != nil {
				return nil, nil, err
			}

		}
	}

	return missingInPerm, missingInDb, nil
}

func (f *Serving) checkPermConsistency(permIdsMap map[string]bool, id string) (consistent bool, err error) {
	var existsInDb bool
	err = DB.Model(Instance{}).Select("count(*) > 0").Where("id = ?", id).Find(&existsInDb).Error
	if err != nil {
		return false, err
	}
	_, existsInPerm := permIdsMap[id]
	return existsInPerm == existsInDb, nil
}
