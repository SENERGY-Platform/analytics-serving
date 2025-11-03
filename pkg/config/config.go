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

package config

import (
	sb_config_hdl "github.com/SENERGY-Platform/go-service-base/config-hdl"
)

type MySQLConfig struct {
	Host     string `json:"host" env_var:"MYSQL_HOST"`
	Port     int    `json:"port" env_var:"MYSQL_PORT"`
	User     string `json:"user" env_var:"MYSQL_USER"`
	Password string `json:"password" env_var:"MYSQL_PW"`
	Database string `json:"database" env_var:"MYSQL_DB"`
}

type InfluxConfig struct {
	Protocol string `json:"protocol" env_var:"INFLUX_DB_PROTO"`
	Host     string `json:"host" env_var:"INFLUX_DB_HOST"`
	Port     int    `json:"port" env_var:"INFLUX_DB_PORT"`
	User     string `json:"user" env_var:"INFLUX_DB_USERNAME"`
	Password string `json:"password" env_var:"INFLUX_DB_PASSWORD"`
}

type LoggerConfig struct {
	Level string `json:"level" env_var:"LOGGER_LEVEL"`
}

type CleanupConfig struct {
	WaitDuration string `json:"wait_duration" env_var:"CLEANUP_WAIT_DURATION"`
	Cron         string `json:"cron" env_var:"CLEANUP_CRON"`
}

type Config struct {
	Logger                 LoggerConfig  `json:"logger" env_var:"LOGGER_CONFIG"`
	URLPrefix              string        `json:"url_prefix" env_var:"URL_PREFIX"`
	ServerPort             int           `json:"server_port" env_var:"SERVER_PORT"`
	Debug                  bool          `json:"debug" env_var:"DEBUG"`
	Driver                 string        `json:"driver" env_var:"DRIVER"`
	MySQL                  MySQLConfig   `json:"mysql" env_var:"MYSQL_CONFIG"`
	MigrationInfo          string        `json:"migration_info" env_var:"MIGRATION_INFO"`
	KafkaBootstrap         string        `json:"kafka_bootstrap" env_var:"KAFKA_BOOTSTRAP"`
	PermissionV2Url        string        `json:"permission_v2_url" env_var:"PERMISSION_V2_URL"`
	PipelineApiUrl         string        `json:"pipeline_api_url" env_var:"PIPELINE_API_ENDPOINT"`
	ImportDeployApiUrl     string        `json:"import_deploy_api_url" env_var:"IMPORT_DEPLOY_API_ENDPOINT"`
	ExportDatabaseIdPrefix string        `json:"export_database_id_prefix" env_var:"EXPORT_DATABASE_ID_PREFIX"`
	CleanupConfig          CleanupConfig `json:"cleanup_config" env_var:"CLEANUP_CONFIG"`
	InfluxConfig           InfluxConfig  `json:"influx_config" env_var:"INFLUX_CONFIG"`
	ApiDocsProviderBaseUrl string        `json:"api_docs_provider_base_url" env_var:"API_DOCS_PROVIDER_BASE_URL"`
}

func New(path string) (*Config, error) {
	cfg := Config{
		ServerPort: 8080,
		URLPrefix:  "",
		Debug:      false,
		Driver:     "ew",
		MySQL: MySQLConfig{
			Host:     "localhost",
			Port:     3306,
			User:     "serving",
			Password: "serving",
			Database: "serving",
		},
		InfluxConfig: InfluxConfig{
			Protocol: "http",
			Host:     "localhost",
			Port:     8086,
			User:     "root",
			Password: "",
		},
		MigrationInfo:          "",
		KafkaBootstrap:         "localhost:9092",
		ExportDatabaseIdPrefix: "",
		CleanupConfig: CleanupConfig{
			WaitDuration: "10s",
			Cron:         "0 1 * * *",
		},
		ApiDocsProviderBaseUrl: "",
	}
	err := sb_config_hdl.Load(&cfg, nil, envTypeParser, nil, path)
	return &cfg, err
}
