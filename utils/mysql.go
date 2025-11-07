package utils

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

// check is a helper function to handle errors
func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func GetConnection(host string, port int, user string, password string) *sql.DB {
	dataSource := fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, host, port)
	db, err := sql.Open("mysql", dataSource)
	check(err)
	err = db.Ping()
	check(err)
	return db
}

// GetMySQLVariable gets a MySQL variable value
// Returns empty string if query fails or variable not found
// This function does not exit on error to allow graceful degradation
func GetMySQLVariable(db *sql.DB, name string) string {
	sql := fmt.Sprintf("select @@%s", name)
	results, err := db.Query(sql)
	if err != nil {
		// Log error but don't exit - allow graceful degradation
		// This is important when user doesn't have permission to query certain variables
		// or when connecting to remote MySQL that may not expose all variables
		return ""
	}
	defer results.Close()

	if results.Next() {
		var val string
		if err := results.Scan(&val); err != nil {
			return ""
		}
		return val
	}
	return ""
}

// GetMySQLConfigFile is deprecated and should not be used.
// We no longer auto-detect MySQL config files to avoid using wrong config file
// (e.g., from another MySQL instance on the same host).
// Users must explicitly specify --defaults-file if they want to use a config file.
// This function is kept for backward compatibility but always returns empty string.
func GetMySQLConfigFile(db *sql.DB) string {
	// Always return empty - no auto-detection
	// Users must explicitly specify --defaults-file
	return ""
}
