package mysql

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

// Connection wraps a MySQL database connection
type Connection struct {
	db *sql.DB
}

// NewConnection creates a new MySQL connection
func NewConnection(host string, port int, user string, password string) (*Connection, error) {
	dataSource := fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, host, port)
	db, err := sql.Open("mysql", dataSource)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Connection{db: db}, nil
}

// Close closes the database connection
func (c *Connection) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// GetVariable retrieves a MySQL system variable value
func (c *Connection) GetVariable(name string) (string, error) {
	query := fmt.Sprintf("select @@%s", name)
	results, err := c.db.Query(query)
	if err != nil {
		return "", err
	}
	defer results.Close()

	for results.Next() {
		var val string
		if err := results.Scan(&val); err != nil {
			return "", err
		}
		return val, nil
	}
	return "", nil
}

// GetDB returns the underlying sql.DB for advanced operations
func (c *Connection) GetDB() *sql.DB {
	return c.db
}

// check is a helper function to handle errors (for backward compatibility)
func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
