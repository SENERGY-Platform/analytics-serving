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
	"log"
	"net"
	"net/url"
	"sync"
	"time"

	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	PostgresPort = 5432
	PostgresUser = "usr"
	// PostgresPassword contains a space, a quote and a backslash, so every test that connects checks the escaping
	// of the connection string.
	PostgresPassword = `p w'\x`
)

func PostgresWithNetwork(ctx context.Context, wg *sync.WaitGroup, dbname string) (conStr string, ip string, port string, err error) {
	log.Println("start postgres container")
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:18",
			Env: map[string]string{
				"POSTGRES_DB":       dbname,
				"POSTGRES_USER":     PostgresUser,
				"POSTGRES_PASSWORD": PostgresPassword,
			},
			ExposedPorts: []string{"5432/tcp"},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("5432/tcp"),
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
		log.Println("DEBUG: remove postgres container", c.Terminate(context.Background()))
	}()

	ip, err = c.ContainerIP(ctx)
	if err != nil {
		return "", "", "", err
	}
	temp, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return "", "", "", err
	}
	port = temp.Port()
	conStr = (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(PostgresUser, PostgresPassword),
		Host:     net.JoinHostPort(ip, "5432"),
		Path:     "/" + dbname,
		RawQuery: "sslmode=disable",
	}).String()

	err = retry(1*time.Minute, func() error {
		log.Println("try postgres conn", ip)
		db, err := sql.Open("postgres", conStr)
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Ping()
	})
	if err != nil {
		log.Println("ERROR:", err)
		return "", "", "", err
	}

	return conStr, ip, port, err
}
