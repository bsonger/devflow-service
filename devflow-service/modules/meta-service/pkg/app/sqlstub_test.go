package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
	"testing"

	"github.com/bsonger/devflow-service/modules/meta-service/pkg/infra/store"
	"github.com/google/uuid"
)

type queuedSQLDriverStub struct {
	mu         sync.Mutex
	execs      []string
	queries    []string
	execQueue  []queuedExecResponse
	queryQueue []queuedQueryResponse
}

type queuedExecResponse struct {
	result driver.Result
	err    error
}

type queuedQueryResponse struct {
	rows driver.Rows
	terr error
}

func (s *queuedSQLDriverStub) Open(name string) (driver.Conn, error) {
	return &queuedSQLConnStub{stub: s}, nil
}

type queuedSQLConnStub struct {
	stub *queuedSQLDriverStub
}

func (c *queuedSQLConnStub) Prepare(query string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *queuedSQLConnStub) Close() error                              { return nil }
func (c *queuedSQLConnStub) Begin() (driver.Tx, error)                 { return nil, driver.ErrSkip }

func (c *queuedSQLConnStub) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.stub.mu.Lock()
	defer c.stub.mu.Unlock()

	c.stub.execs = append(c.stub.execs, query)
	if len(c.stub.execQueue) == 0 {
		return driver.RowsAffected(1), nil
	}
	resp := c.stub.execQueue[0]
	c.stub.execQueue = c.stub.execQueue[1:]
	if resp.result == nil && resp.err == nil {
		resp.result = driver.RowsAffected(1)
	}
	return resp.result, resp.err
}

func (c *queuedSQLConnStub) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.stub.mu.Lock()
	defer c.stub.mu.Unlock()

	c.stub.queries = append(c.stub.queries, query)
	if len(c.stub.queryQueue) == 0 {
		return &queuedSQLRowsStub{}, nil
	}
	resp := c.stub.queryQueue[0]
	c.stub.queryQueue = c.stub.queryQueue[1:]
	if resp.rows == nil && resp.terr == nil {
		resp.rows = &queuedSQLRowsStub{}
	}
	return resp.rows, resp.terr
}

type queuedSQLRowsStub struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *queuedSQLRowsStub) Columns() []string { return r.columns }
func (r *queuedSQLRowsStub) Close() error      { return nil }
func (r *queuedSQLRowsStub) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func openQueuedSQLStub(t *testing.T, stub *queuedSQLDriverStub) *sql.DB {
	t.Helper()
	driverName := "app_service_sql_queue_stub_" + uuid.NewString()
	sql.Register(driverName, stub)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	store.InitPostgres(db)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func queuedRows(columns []string, rows ...[]driver.Value) driver.Rows {
	return &queuedSQLRowsStub{columns: columns, rows: rows}
}
