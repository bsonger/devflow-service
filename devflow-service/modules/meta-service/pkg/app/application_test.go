package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
	"testing"

	loggingx "github.com/bsonger/devflow-service/internal/platform/logger"
	"github.com/bsonger/devflow-service/modules/meta-service/pkg/infra/store"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestUpdateActiveImageUpdatesApplicationWithoutImageLookup(t *testing.T) {
	loggingx.Logger = zap.NewNop()

	stub := &sqlDriverStub{
		execResult: driver.RowsAffected(1),
	}

	db := openSQLStub(t, stub)
	store.InitPostgres(db)

	err := NewApplicationService().UpdateActiveImage(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("UpdateActiveImage returned error: %v", err)
	}

	if len(stub.execs) != 1 {
		t.Fatalf("exec count = %d, want 1", len(stub.execs))
	}
	if len(stub.queries) != 0 {
		t.Fatalf("query count = %d, want 0", len(stub.queries))
	}
}

func TestUpdateActiveImageReturnsNotFoundWhenApplicationMissing(t *testing.T) {
	loggingx.Logger = zap.NewNop()

	stub := &sqlDriverStub{
		execResult: driver.RowsAffected(0),
	}

	db := openSQLStub(t, stub)
	store.InitPostgres(db)

	err := NewApplicationService().UpdateActiveImage(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("UpdateActiveImage should fail when no application row was updated")
	}
}

type sqlDriverStub struct {
	mu         sync.Mutex
	execs      []string
	queries    []string
	execResult driver.Result
	execErr    error
	queryRows  driver.Rows
	queryErr   error
}

func (s *sqlDriverStub) Open(name string) (driver.Conn, error) {
	return &sqlConnStub{stub: s}, nil
}

type sqlConnStub struct {
	stub *sqlDriverStub
}

func (c *sqlConnStub) Prepare(query string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *sqlConnStub) Close() error                              { return nil }
func (c *sqlConnStub) Begin() (driver.Tx, error)                 { return nil, driver.ErrSkip }

func (c *sqlConnStub) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.stub.mu.Lock()
	defer c.stub.mu.Unlock()
	c.stub.execs = append(c.stub.execs, query)
	return c.stub.execResult, c.stub.execErr
}

func (c *sqlConnStub) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.stub.mu.Lock()
	defer c.stub.mu.Unlock()
	c.stub.queries = append(c.stub.queries, query)
	if c.stub.queryRows != nil || c.stub.queryErr != nil {
		return c.stub.queryRows, c.stub.queryErr
	}
	return &sqlRowsStub{}, nil
}

type sqlRowsStub struct{}

func (r *sqlRowsStub) Columns() []string           { return nil }
func (r *sqlRowsStub) Close() error                { return nil }
func (r *sqlRowsStub) Next(_ []driver.Value) error { return io.EOF }

func openSQLStub(t *testing.T, stub *sqlDriverStub) *sql.DB {
	t.Helper()
	driverName := "app_service_sql_stub_" + uuid.NewString()
	sql.Register(driverName, stub)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
