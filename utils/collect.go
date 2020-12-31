package utils

import (
	"database/sql"
	"fmt"
)

func CollectVariableFromMySQLServer(db *sql.DB) map[string]string {
	items := []string {
		"version",
		"gtid_mode",
		"enforce_gtid_consistency",
		"innodb_data_file_path",
		"server_id",
		"log_bin",
	}
	result := make(map[string]string)
	fmt.Println("获取参数中...")
	for _, item := range items {
		val := GetMySQLVariable(db, item)
		fmt.Printf("\t%s=%s\n", item, val)
		result[item] = val
	}
	fmt.Println()
	return result
}
