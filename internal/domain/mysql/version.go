package mysql

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
)

// Version represents MySQL version
type Version struct {
	Major int
	Minor int
	Micro int
}

// ParseVersion parses a MySQL version string (e.g., "8.0.26-log")
func ParseVersion(versionStr string) (Version, error) {
	header := strings.Split(versionStr, "-")[0]
	parts := strings.Split(header, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid MySQL version format: %s", versionStr)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, err
	}
	micro, err := strconv.Atoi(parts[2])
	if err != nil {
		return Version{}, err
	}

	return Version{
		Major: major,
		Minor: minor,
		Micro: micro,
	}, nil
}

// CheckXtraBackupCompatibility checks if xtrabackup is compatible with MySQL version
func CheckXtraBackupCompatibility(mysqlVer Version) {
	cmd := exec.Command("xtrabackup", "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := i18n.Sprintf("[Error] Cannot execute xtrabackup --version, please confirm that Percona XtraBackup is installed and in PATH")
		i18n.Printf(color.RedString("%s\n", msg))
		return
	}
	versionStr := string(out)

	// Extract full version number (e.g., 8.0.34-29)
	re := regexp.MustCompile(`([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9]+))?`)
	match := re.FindStringSubmatch(versionStr)
	var xtrabackupVerParts [4]int
	if len(match) >= 4 {
		xtrabackupVerParts[0], _ = strconv.Atoi(match[1])
		xtrabackupVerParts[1], _ = strconv.Atoi(match[2])
		xtrabackupVerParts[2], _ = strconv.Atoi(match[3])
		if len(match) >= 5 && match[4] != "" {
			xtrabackupVerParts[3], _ = strconv.Atoi(match[4])
		}
	}

	// Verification
	if mysqlVer.Major == 5 && mysqlVer.Minor == 7 {
		if xtrabackupVerParts[0] == 2 && xtrabackupVerParts[1] == 4 {
			msg := i18n.Sprintf("[OK] MySQL 5.7 detected xtrabackup 2.4 version, compatible")
			i18n.Printf(color.GreenString("%s\n", msg))
		} else {
			msg := i18n.Sprintf("[Warning] MySQL 5.7 recommends xtrabackup 2.4, but detected version: %d.%d", xtrabackupVerParts[0], xtrabackupVerParts[1])
			i18n.Printf(color.RedString("%s\n", msg))
		}
	} else if mysqlVer.Major == 8 && mysqlVer.Minor == 0 {
		if xtrabackupVerParts[0] == 8 && xtrabackupVerParts[1] == 0 {
			msg := i18n.Sprintf("[OK] MySQL 8.0 detected xtrabackup 8.0 version, compatible")
			i18n.Printf(color.GreenString("%s\n", msg))
			if XtrabackupVersionGreaterOrEqual(xtrabackupVerParts, [4]int{8, 0, 34, 29}) {
				hint := i18n.Sprintf("[Hint] Detected xtrabackup 8.0.34-29 or later, default zstd compression may cause recovery to fail.")
				i18n.Printf(color.YellowString("%s\n", hint))
			}
		} else {
			msg := i18n.Sprintf("[Warning] MySQL 8.0 recommends xtrabackup 8.0, but detected version: %d.%d", xtrabackupVerParts[0], xtrabackupVerParts[1])
			i18n.Printf(color.RedString("%s\n", msg))
		}
	}
}

// GetXtrabackupVersion extracts xtrabackup major.minor.patch-revision four-part version number
func GetXtrabackupVersion() [4]int {
	cmd := exec.Command("xtrabackup", "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return [4]int{0, 0, 0, 0}
	}
	versionStr := string(out)
	re := regexp.MustCompile(`([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9]+))?`)
	match := re.FindStringSubmatch(versionStr)
	var xtrabackupVerParts [4]int
	if len(match) >= 4 {
		xtrabackupVerParts[0], _ = strconv.Atoi(match[1])
		xtrabackupVerParts[1], _ = strconv.Atoi(match[2])
		xtrabackupVerParts[2], _ = strconv.Atoi(match[3])
		if len(match) >= 5 && match[4] != "" {
			xtrabackupVerParts[3], _ = strconv.Atoi(match[4])
		}
	}
	return xtrabackupVerParts
}

// XtrabackupVersionGreaterOrEqual compares major.minor.patch-revision four-part version number
func XtrabackupVersionGreaterOrEqual(v, target [4]int) bool {
	for i := 0; i < 4; i++ {
		if v[i] > target[i] {
			return true
		} else if v[i] < target[i] {
			return false
		}
	}
	return true // Equal
}
