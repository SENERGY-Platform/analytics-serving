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

package model

import (
	"github.com/satori/go.uuid"
)

type Instances []Instance

type Instance struct {
	ID               uuid.UUID `gorm:"primary_key;type:char(36);column:id"`
	Name             string    `gorm:"type:varchar(255)"`
	Description      string    `gorm:"type:varchar(255)"`
	EntityName       string    `gorm:"type:varchar(255)"`
	ServiceName      string    `gorm:"type:varchar(255)"`
	Topic            string    `gorm:"type:varchar(255)"`
	Database         string    `gorm:"type:varchar(255)"`
	Measurement      string    `gorm:"type:varchar(255)"`
	Filter           string    `gorm:"type:varchar(255)"`
	FilterType       string    `gorm:"type:varchar(255)"`
	TimePath         string    `gorm:"type:varchar(255)"`
	UserId           string    `gorm:"type:varchar(255)"`
	RancherServiceId string    `gorm:"type:varchar(255)"`
	Values           []Value   `gorm:"foreignkey:InstanceID;association_foreignkey:ID;PRELOAD:true"`
}

type Value struct {
	InstanceID uuid.UUID `gorm:"type:char(36)"`
	Name       string    `gorm:"type:varchar(255)"`
	Type       string    `gorm:"type:varchar(255)"`
	Path       string    `gorm:"type:varchar(255)"`
}
