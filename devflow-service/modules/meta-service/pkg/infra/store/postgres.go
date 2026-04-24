package store

import (
	"database/sql"

	platformdb "github.com/bsonger/devflow-service/internal/platform/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var db *sql.DB

func InitPostgres(d *sql.DB) {
	db = d
	platformdb.InitPostgres(d)
}

func DB() *sql.DB {
	if db != nil {
		return db
	}
	return platformdb.Postgres()
}

func ApplyPool(conn *sql.DB, maxOpen, maxIdle, lifetimeMinutes int) {
	platformdb.ApplyPool(conn, maxOpen, maxIdle, lifetimeMinutes)
}
