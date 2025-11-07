package utils

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

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

// GetMySQLConfigFile attempts to find MySQL configuration file (my.cnf)
// Returns the path if found, empty string if not found
// This function gracefully handles cases where MySQL variables cannot be queried
// (e.g., insufficient permissions, remote connections, etc.)
func GetMySQLConfigFile(db *sql.DB) string {
	// Common MySQL config file locations (checked first, before querying MySQL)
	commonPaths := []string{
		"/etc/my.cnf",
		"/etc/mysql/my.cnf",
		"/usr/etc/my.cnf",
		"/var/lib/mysql/my.cnf",
		"~/.my.cnf",
		"/etc/my.cnf.d/mysql-server.cnf",
	}

	// Try to get basedir from MySQL (gracefully handle query failures)
	// If query fails (e.g., no permission), we continue with common paths only
	if db != nil {
		basedir := GetMySQLVariable(db, "basedir")
		if basedir != "" {
			commonPaths = append([]string{
				filepath.Join(basedir, "my.cnf"),
				filepath.Join(basedir, "my.ini"),
				filepath.Join(basedir, "etc", "my.cnf"),
			}, commonPaths...)
		}

		// Try to get datadir from MySQL (gracefully handle query failures)
		datadir := GetMySQLVariable(db, "datadir")
		if datadir != "" {
			// Check parent directory of datadir
			parentDir := filepath.Dir(datadir)
			commonPaths = append([]string{
				filepath.Join(parentDir, "my.cnf"),
				filepath.Join(parentDir, "my.ini"),
			}, commonPaths...)
		}
	}

	// Check each path
	for _, path := range commonPaths {
		// Handle ~ expansion
		if path[0] == '~' {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				path = filepath.Join(homeDir, path[1:])
			}
		}

		// Check if file exists and is readable
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}

	return ""
}
