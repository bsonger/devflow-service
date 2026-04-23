package domain

import "testing"

func TestPostgresConfigContract(t *testing.T) {
	cfg := PostgresConfig{
		DSN:                    "postgres://devflow:devflow@localhost:5432/devflow?sslmode=disable",
		MaxOpenConns:           12,
		MaxIdleConns:           6,
		ConnMaxLifetimeMinutes: 30,
	}

	if cfg.DSN == "" {
		t.Fatal("PostgresConfig should expose DSN")
	}
	if cfg.MaxOpenConns != 12 {
		t.Fatalf("PostgresConfig.MaxOpenConns = %d, want 12", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 6 {
		t.Fatalf("PostgresConfig.MaxIdleConns = %d, want 6", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetimeMinutes != 30 {
		t.Fatalf("PostgresConfig.ConnMaxLifetimeMinutes = %d, want 30", cfg.ConnMaxLifetimeMinutes)
	}
}
