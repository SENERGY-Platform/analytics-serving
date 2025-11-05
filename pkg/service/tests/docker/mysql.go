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

package docker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func MySqlWithNetwork(ctx context.Context, wg *sync.WaitGroup, dbname string) (conStr string, ip string, port string, err error) {
	log.Println("start mysql container")
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "mysql:5.7",
			Env: map[string]string{
				"MYSQL_ROOT_PASSWORD": "pw",
				"MYSQL_DATABASE":      dbname,
				"MYSQL_PASSWORD":      "pw",
				"MYSQL_USER":          "usr",
			},
			ExposedPorts: []string{"3306/tcp"},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("3306/tcp"),
			),
		},
		Started: true,
	})
	if err != nil {
		return "", "", "", err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		log.Println("DEBUG: remove mysql container", c.Terminate(context.Background()))
	}()

	ip, err = c.ContainerIP(ctx)
	if err != nil {
		return "", "", "", err
	}
	temp, err := c.MappedPort(ctx, "3306/tcp")
	if err != nil {
		return "", "", "", err
	}
	port = temp.Port()
	conStr = fmt.Sprintf("usr:pw@(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", ip, "3306", dbname)

	err = retry(1*time.Minute, func() error {
		log.Println("try mysql conn", conStr)
		db, err := sql.Open("mysql", conStr)
		if err != nil {
			return err
		}
		err = db.Ping()
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Println("ERROR:", err)
		return "", "", "", err
	}

	return conStr, ip, port, err
}
