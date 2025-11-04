package utils

import (
	"database/sql"

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
	i18n.Printf("Get parameter for checking...\n")
	for _, item := range items {
		val := GetMySQLVariable(db, item)
		i18n.Printf("\t%s=%s\n", item, val)
		result[item] = val
	}
	i18n.Printf("\n")
	return result
}
