package mysql

import (
	"fmt"

	"github.com/gioco-play/easy-i18n/i18n"
)

// CollectVariables collects important MySQL system variables for checking
func (c *Connection) CollectVariables() (map[string]string, error) {
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
		val, err := c.GetVariable(item)
		if err != nil {
			return nil, err
		}
		i18n.Printf("\t%s=%s\n", item, val)
		result[item] = val
	}

	i18n.Printf("\n")
	return result, nil
}

// GetDatadir queries MySQL for the datadir path
func (c *Connection) GetDatadir() (string, error) {
	datadir, err := c.GetVariable("datadir")
	if err != nil {
		return "", err
	}
	if datadir == "" {
		return "", fmt.Errorf("failed to get datadir from MySQL")
	}
	return datadir, nil
}
