package db

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var postgres *sql.DB

func InitPostgres(conn *sql.DB) {
	postgres = conn
}

func Postgres() *sql.DB {
	if postgres == nil {
		panic("postgres store not initialized")
	}
	return postgres
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
