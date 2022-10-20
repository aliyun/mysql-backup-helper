package utils

import (
	"database/sql"
	"fmt"

	"github.com/gioco-play/easy-i18n/i18n"
)

func CollectVariableFromMySQLServer(db *sql.DB) map[string]string {
	items := []string{
		"version",
		"gtid_mode",
		"enforce_gtid_consistency",
		"innodb_data_file_path",
		"server_id",
		"log_bin",
	}
	result := make(map[string]string)
	i18n.Printf("获取参数中...")
	fmt.Println()
	for _, item := range items {
		val := GetMySQLVariable(db, item)
		fmt.Printf("\t%s=%s\n", item, val)
		result[item] = val
	}
	fmt.Println()
	return result
}
