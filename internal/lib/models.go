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
	"github.com/google/uuid"
	"time"
)

type Instances []Instance

type Instance struct {
	ID               uuid.UUID `gorm:"primary_key;type:char(36);column:id"`
	Name             string    `gorm:"type:varchar(255)"`
	Description      string    `gorm:"type:varchar(255)"`
	EntityName       string    `gorm:"type:varchar(255)"`
	ServiceName      string    `gorm:"type:varchar(255)"`
	Topic            string    `gorm:"type:varchar(255)"`
	ApplicationId    uuid.UUID `gorm:"type:char(36)"`
	Database         string    `gorm:"type:varchar(255)"`
	Measurement      string    `gorm:"type:varchar(255)"`
	Filter           string    `gorm:"type:varchar(255)"`
	FilterType       string    `gorm:"type:varchar(255)"`
	TimePath         string    `gorm:"type:varchar(255)"`
	TimePrecision    *string   `gorm:"type:varchar(255)"`
	UserId           string    `gorm:"type:varchar(255)"`
	Generated        bool      `gorm:"type:bool;DEFAULT:false"`
	RancherServiceId string    `gorm:"type:varchar(255)"`
	Offset           string    `gorm:"type:varchar(255)"`
	DatabaseType     string    `gorm:"type:varchar(255)"`
	ExportDatabaseID string    `gorm:"type:varchar(255)"`
	TimestampFormat  string    `gorm:"type:varchar(255)"`
	Values           []Value   `gorm:"foreignkey:InstanceID;association_foreignkey:ID"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Value struct {
	InstanceID uuid.UUID `gorm:"type:char(36)"`
	Name       string    `gorm:"type:varchar(255)"`
	Type       string    `gorm:"type:varchar(255)"`
	Path       string    `gorm:"type:varchar(255)"`
	Tag        bool      `gorm:"type:bool;DEFAULT:false"`
}

type ExportDatabase struct {
	ID            string `gorm:"primary_key;type:varchar(255);column:id"`
	Name          string `gorm:"type:varchar(255)"`
	Description   string `gorm:"type:varchar(255)"`
	Type          string `gorm:"type:varchar(255)"`
	Internal      bool   `gorm:"type:bool;DEFAULT:false"`
	Url           string `gorm:"type:varchar(255)"`
	EwFilterTopic string `gorm:"type:varchar(255)"`
	UserId        string `gorm:"type:varchar(255)"`
}
