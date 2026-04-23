package store

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var db *sql.DB

func InitPostgres(d *sql.DB) {
	db = d
}

func DB() *sql.DB {
	if db == nil {
		panic("postgres store not initialized")
	}
	return db
}

func ApplyPool(conn *sql.DB, maxOpen, maxIdle, lifetimeMinutes int) {
	if maxOpen > 0 {
		conn.SetMaxOpenConns(maxOpen)
	}
	if maxIdle > 0 {
		conn.SetMaxIdleConns(maxIdle)
	}
	if lifetimeMinutes > 0 {
		conn.SetConnMaxLifetime(time.Duration(lifetimeMinutes) * time.Minute)
	}
}
