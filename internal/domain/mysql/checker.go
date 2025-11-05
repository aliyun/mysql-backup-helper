package mysql

import (
	"backup-helper/internal/config"
	"backup-helper/internal/pkg/format"
	"fmt"
	"strings"

	"github.com/gioco-play/easy-i18n/i18n"
)

// Checker handles MySQL configuration checks
type Checker struct {
	conn *Connection
}

// NewChecker creates a new checker
func NewChecker(conn *Connection) *Checker {
	return &Checker{conn: conn}
}

// CheckAll performs all checks
func (ch *Checker) CheckAll(cfg *config.Config) error {
	variables, err := ch.conn.CollectVariables()
	if err != nil {
		return err
	}

	ch.checkVersion(variables["version"], cfg)
	ch.checkBackup(variables)
	ch.checkReplication(variables)

	return nil
}

// checkVersion checks MySQL version
func (ch *Checker) checkVersion(versionStr string, cfg *config.Config) {
	i18n.Printf("Checking MySQL Server Version...\n")

	ver, err := ParseVersion(versionStr)
	if err != nil {
		format.Output(i18n.Sprintf("Version"), versionStr, i18n.Sprintf("Invalid version format"), false)
		return
	}

	cfg.MysqlVersion = config.MySQLVersion{
		Major: ver.Major,
		Minor: ver.Minor,
		Micro: ver.Micro,
	}

	checkItem := i18n.Sprintf("Version")
	if ver.Major == 5 && ver.Minor == 7 {
		format.Output(checkItem, versionStr, "", true)
	} else if ver.Major == 8 && ver.Minor == 0 && ver.Micro <= 36 {
		format.Output(checkItem, versionStr, "", true)
	} else {
		format.Output(checkItem, versionStr, i18n.Sprintf("maybe incompatible"), false)
		i18n.Printf("\tYour MySQL Server version may newer than version that provided On Alibaba Cloud, data file probably incompatible, read doc online for more info.\n")
	}
}

// checkInnodbFilePath checks innodb_data_file_path parameter
func (ch *Checker) checkInnodbFilePath(variables map[string]string) {
	key := "innodb_data_file_path"
	val := variables[key]
	tokens := strings.Split(val, ";")
	checkValue := key + "=" + val
	if len(tokens) > 1 {
		format.Output(i18n.Sprintf("Parameter"), checkValue, i18n.Sprintf("Multiple parameters are not supported"), false)
	} else {
		filename := strings.Split(tokens[0], ":")[0]
		if filename == "ibdata1" {
			format.Output(i18n.Sprintf("Parameter"), checkValue, "", true)
		} else {
			format.Output(i18n.Sprintf("Parameter"), checkValue, i18n.Sprintf("Recommended parameter: ibdata1"), false)
		}
	}
}

// checkBackup checks backup-related parameters
func (ch *Checker) checkBackup(variables map[string]string) {
	i18n.Printf("Checking backup related parameters...\n")
	ch.checkInnodbFilePath(variables)
	i18n.Printf("Backup related parameters checked...\n")
}

// checkReplication checks replication parameters
func (ch *Checker) checkReplication(variables map[string]string) {
	i18n.Printf("Checking replication parameters (these parameters affect master-slave replication, but do not affect backup) ...\n")

	miss := []string{"server_id", "log_bin"}
	for _, m := range miss {
		checkMissVariable(m, variables[m])
	}

	items := []struct {
		name  string
		value string
	}{
		{"gtid_mode", "ON"},
		{"enforce_gtid_consistency", "ON"},
	}
	for _, item := range items {
		if userVal, ok := variables[item.name]; ok {
			checkValue := fmt.Sprintf("%s=%s", item.name, userVal)
			if userVal != item.value {
				suggest := i18n.Sprintf("Recommended parameter: %s", item.value)
				fmt.Println()
				format.Output(i18n.Sprintf("Parameter"), checkValue, suggest, false)
			} else {
				format.Output(i18n.Sprintf("Parameter"), checkValue, "", true)
			}
		}
	}

	i18n.Printf("Replication parameter check completed")
	fmt.Println()
}

// checkMissVariable checks if a required variable is set
func checkMissVariable(key, value string) {
	checkValue := key + "=" + value
	if value == "0" {
		format.Output(i18n.Sprintf("Parameter"), checkValue, i18n.Sprintf("Parameter not set"), false)
	} else {
		format.Output(i18n.Sprintf("Parameter"), checkValue, "", true)
	}
}
