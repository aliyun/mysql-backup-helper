package utils

import (
	"database/sql"

	"github.com/gioco-play/easy-i18n/i18n"
)

func CollectVariableFromMySQLServer(db *sql.DB) map[string]string {
	return CollectVariableFromMySQLServerSilent(db, false)
}

// CollectVariableFromMySQLServerSilent collects MySQL variables, with optional silent mode
func CollectVariableFromMySQLServerSilent(db *sql.DB, silent bool) map[string]string {
	items := []string{
		"version",
		"gtid_mode",
		"enforce_gtid_consistency",
		"innodb_data_file_path",
		"server_id",
		"log_bin",
	}
	result := make(map[string]string)
	if !silent {
		i18n.Printf("Get parameter for checking...\n")
	}
	for _, item := range items {
		val := GetMySQLVariable(db, item)
		if !silent {
			i18n.Printf("\t%s=%s\n", item, val)
		}
		result[item] = val
	}
	if !silent {
		i18n.Printf("\n")
	}
	return result
}
