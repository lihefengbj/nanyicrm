// initdb creates the application database if it does not exist yet.
// It connects without a database name, so it works before the schema exists.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"

	_ "github.com/go-sql-driver/mysql"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
)

var dbNameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func main() {
	cfg := config.Load()

	dbName := os.Getenv("INIT_DB_NAME")
	if dbName == "" {
		dbName = "nanyicrm"
	}
	if !dbNameRe.MatchString(dbName) {
		log.Fatalf("invalid database name %q", dbName)
	}

	// Strip the database name from the DSN: user:pwd@tcp(host:port)/db?params
	dsn := cfg.DefaultMySQL().DSN
	re := regexp.MustCompile(`\)/[^/?]*`)
	dsn = re.ReplaceAllString(dsn, ")/")

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName,
	)); err != nil {
		log.Fatalf("create database: %v", err)
	}
	log.Printf("database %s ready", dbName)
}

