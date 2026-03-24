package proxysql

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/proxysql/golib/log"
)

// Client manages a connection to ProxySQL's Admin interface.
// The Admin interface speaks the MySQL protocol, so we use a standard MySQL driver.
type Client struct {
	dsn     string
	address string
	port    int
}

// NewClient creates a new ProxySQL Admin client.
// Returns nil if address is empty (unconfigured — all operations become no-ops).
func NewClient(address string, port int, user, password string, useTLS bool) *Client {
	if address == "" {
		return nil
	}
	base := fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, address, port)
	// interpolateParams=true: ProxySQL Admin does not support prepared statements (COM_STMT_PREPARE).
	// This tells the Go MySQL driver to interpolate parameters client-side.
	params := "interpolateParams=true&timeout=1s&readTimeout=1s&writeTimeout=1s"
	dsn := base + "?" + params
	if useTLS {
		dsn = base + "?tls=true&" + params
	}
	return &Client{
		dsn:     dsn,
		address: address,
		port:    port,
	}
}

// openDB opens a fresh connection to ProxySQL Admin.
// Callers must close the returned *sql.DB.
func (c *Client) openDB() (*sql.DB, error) {
	db, err := sql.Open("mysql", c.dsn)
	if err != nil {
		return nil, fmt.Errorf("proxysql: failed to open connection to %s:%d: %v", c.address, c.port, err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// Exec executes an admin command against ProxySQL.
func (c *Client) Exec(query string, args ...interface{}) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("proxysql: exec failed: %v", err)
	}
	return nil
}

// Query executes a query and returns rows. Caller must close both rows and db.
func (c *Client) Query(query string, args ...interface{}) (*sql.Rows, *sql.DB, error) {
	db, err := c.openDB()
	if err != nil {
		return nil, nil, err
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("proxysql: query failed: %v", err)
	}
	return rows, db, nil
}

// Ping verifies the connection to ProxySQL Admin.
func (c *Client) Ping() error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("proxysql: ping failed on %s:%d: %v", c.address, c.port, err)
	}
	log.Infof("proxysql: successfully connected to Admin at %s:%d", c.address, c.port)
	return nil
}
