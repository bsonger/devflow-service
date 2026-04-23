package store

import (
	"database/sql"
	"testing"
)

func TestInitPostgresAndDB(t *testing.T) {
	previous := db
	db = nil
	t.Cleanup(func() { db = previous })

	conn := &sql.DB{}
	InitPostgres(conn)

	if got := DB(); got != conn {
		t.Fatalf("DB() returned %p, want %p", got, conn)
	}
}

func TestApplyPoolAcceptsPositiveValues(t *testing.T) {
	conn := &sql.DB{}
	ApplyPool(conn, 10, 5, 15)
}
