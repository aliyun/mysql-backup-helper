package mysql

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetDatadirFromMySQL queries MySQL for the datadir path
func GetDatadirFromMySQL(db *sql.DB) (string, error) {
	datadir := GetMySQLVariable(db, "datadir")
	if datadir == "" {
		return "", fmt.Errorf("failed to get datadir from MySQL")
	}
	return datadir, nil
}

// CalculateBackupSize calculates the size of files that xtrabackup would backup
func CalculateBackupSize(datadir string) (int64, error) {
	var totalSize int64

	err := filepath.WalkDir(datadir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		base := filepath.Base(path)

		shouldBackup := false

		if strings.HasPrefix(base, "ibdata") || strings.HasPrefix(base, "ib_logfile") || strings.HasPrefix(base, "ibtmp") {
			shouldBackup = true
		}

		if ext == ".ibd" || ext == ".frm" || ext == ".opt" {
			shouldBackup = true
		}

		if ext == ".myd" || ext == ".myi" {
			shouldBackup = true
		}

		if ext == ".csm" || ext == ".csv" {
			shouldBackup = true
		}

		// binlog and relay log are NOT backed up by xtrabackup, so exclude them
		if strings.HasPrefix(base, "binlog") || strings.HasPrefix(base, "relay-bin") {
			shouldBackup = false
		}

		if shouldBackup {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			totalSize += info.Size()
		}

		return nil
	})

	return totalSize, err
}

// GetFileSize gets the size of a file
func GetFileSize(filepath string) (int64, error) {
	info, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
