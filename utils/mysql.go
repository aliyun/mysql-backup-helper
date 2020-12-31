package utils

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"

)

func GetConnection(host string, port int, user string, password string) *sql.DB {
	dataSource := fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, host, port)
	db, err := sql.Open("mysql", dataSource)
	check(err)
	err = db.Ping()
	check(err)
	return db
}

func GetMySQLVariable(db *sql.DB, name string) string {
	sql := fmt.Sprintf("select @@%s", name)
	results, err := db.Query(sql)
	check(err)
	for results.Next() {
		var val string
		err = results.Scan(&val)
		check(err)
		return val
	}
	return ""
}