package check

import (
	"backup-helper/internal/config"
	"backup-helper/internal/utils"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
)

type Option struct {
	name  string
	value string
}

type Version struct {
	major int
	minor int
	micro int
}

func Check(options map[string]string, cfg *config.Config) {
	checkVersion(options["version"], cfg)
	checkBackup(options)
	checkReplication(options)
}

// output is a helper function to output check results
// Uses table format similar to original implementation
func output(item, msg, suggest string, ok bool) {
	tip := color.GreenString("[DONE]")
	if !ok {
		tip = color.RedString("[ERROR]")
	}
	// Simple output format (without goterm table for now)
	if suggest != "" {
		fmt.Printf("\t%s\t%s\t%s\t%s\n", item, msg, suggest, tip)
	} else {
		fmt.Printf("\t%s\t%s\t\t%s\n", item, msg, tip)
	}
}

func checkVersion(value string, cfg *config.Config) {
	i18n.Printf("Checking MySQL Server Version...\n")
	v := getVersion(value)
	// Convert local Version to config.Version
	cfg.MysqlVersion = config.Version{
		Major: v.major,
		Minor: v.minor,
	}
	checkItem := i18n.Sprintf("Version")
	if v.major == 5 && v.minor == 7 {
		output(checkItem, value, "", true)
	} else if v.major == 8 && v.minor == 0 && v.micro <= 36 {
		output(checkItem, value, "", true)
	} else {
		output(checkItem, value, i18n.Sprintf("maybe incompatible"), false)
		i18n.Printf("\tYour MySQL Server version may newer than version that provided On Alibaba Cloud, data file probably incompatible, read doc online for more info.\n")
	}
}

func checkInnodbFilePath(options map[string]string) {
	// file_name:file_size[:autoextend[:max:max_file_size]]
	key := "innodb_data_file_path"
	val := options[key]
	tokens := strings.Split(val, ";")
	checkValue := key + "=" + val
	if len(tokens) > 1 {
		output(i18n.Sprintf("Parameter"), checkValue, i18n.Sprintf("Multiple parameters are not supported"), false)
	} else {
		filename := strings.Split(tokens[0], ":")[0]
		if filename == "ibdata1" {
			output(i18n.Sprintf("Parameter"), checkValue, "", true)
		} else {
			output(i18n.Sprintf("Parameter"), checkValue, i18n.Sprintf("Recommended parameter: ibdata1"), false)
		}
	}
}

func checkBackup(options map[string]string) {
	i18n.Printf("Checking backup related parameters...\n")
	checkInnodbFilePath(options)
	i18n.Printf("Backup related parameters checked...\n")
}

func checkReplication(options map[string]string) {
	i18n.Printf("Checking replication parameters (these parameters affect master-slave replication, but do not affect backup) ...\n")

	miss := []string{"server_id", "log_bin"}
	for _, m := range miss {
		checkMissVariable(m, options[m])
	}

	items := []Option{
		{"gtid_mode", "ON"},
		{"enforce_gtid_consistency", "ON"},
	}
	for _, item := range items {
		if userVal, ok := options[item.name]; ok {
			checkValue := fmt.Sprintf("%s=%s", item.name, userVal)
			if userVal != item.value {
				suggest := i18n.Sprintf("Recommended parameter: %s", item.value)
				fmt.Println()
				output(i18n.Sprintf("Parameter"), checkValue, suggest, false)
			} else {
				output(i18n.Sprintf("Parameter"), checkValue, "", true)
			}
		}
	}

	i18n.Printf("Replication parameter check completed")
	fmt.Println()
}

func checkMissVariable(key, value string) {
	checkValue := key + "=" + value
	if value == "0" {
		output(i18n.Sprintf("Parameter"), checkValue, i18n.Sprintf("Parameter not set"), false)
	} else {
		output(i18n.Sprintf("Parameter"), checkValue, "", true)
	}
}

func getVersion(value string) Version {
	header := strings.Split(value, "-")[0]
	vers := strings.Split(header, ".")
	if len(vers) != 3 {
		panic("MySQL Version error: " + value)
	}
	major, _ := strconv.Atoi(vers[0])
	minor, _ := strconv.Atoi(vers[1])
	micro, _ := strconv.Atoi(vers[2])
	return Version{major, minor, micro}
}

func CheckXtraBackupVersion(mysqlVer config.Version, cfg *config.Config) {
	// Resolve xtrabackup path
	xtrabackupPath, _, err := utils.ResolveXtrabackupPath(cfg.XtrabackupPath, false)
	if err != nil {
		msg := fmt.Sprintf("[Error] Cannot resolve xtrabackup path: %v", err)
		i18n.Printf(color.RedString("%s\n", msg))
		return
	}

	cmd := exec.Command(xtrabackupPath, "--version")
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
		// MySQL 5.7 only supports xtrabackup 2.4
		if xtrabackupVerParts[0] == 2 && xtrabackupVerParts[1] == 4 {
			msg := i18n.Sprintf("[OK] MySQL 5.7 detected xtrabackup 2.4 version, compatible")
			i18n.Printf(color.GreenString("%s\n", msg))
		} else {
			msg := i18n.Sprintf("[ERROR] MySQL 5.7 only supports xtrabackup 2.4, but detected %d.%d. Please use xtrabackup 2.4 for MySQL 5.7", xtrabackupVerParts[0], xtrabackupVerParts[1])
			i18n.Printf(color.RedString("%s\n", msg))
			os.Exit(1)
		}
	} else if mysqlVer.Major == 8 && mysqlVer.Minor == 0 {
		// MySQL 8.0 only supports xtrabackup 8.0
		if xtrabackupVerParts[0] == 8 && xtrabackupVerParts[1] == 0 {
			msg := i18n.Sprintf("[OK] MySQL 8.0 detected xtrabackup 8.0 version, compatible")
			i18n.Printf(color.GreenString("%s\n", msg))
			if XtrabackupVersionGreaterOrEqual(xtrabackupVerParts, [4]int{8, 0, 34, 29}) {
				hint := i18n.Sprintf("[Hint] Detected xtrabackup 8.0.34-29 or later, default zstd compression may cause recovery to fail.")
				i18n.Printf(color.YellowString("%s\n", hint))
			}
		} else {
			msg := i18n.Sprintf("[ERROR] MySQL 8.0 only supports xtrabackup 8.0, but detected %d.%d. Please use xtrabackup 8.0 for MySQL 8.0", xtrabackupVerParts[0], xtrabackupVerParts[1])
			i18n.Printf(color.RedString("%s\n", msg))
			os.Exit(1)
		}
	} else {
		// Other MySQL versions (e.g., 5.6, 8.1, 8.4, etc.) are not supported
		msg := i18n.Sprintf("[ERROR] MySQL %d.%d is not supported. Only MySQL 5.7 and 8.0 are supported", mysqlVer.Major, mysqlVer.Minor)
		i18n.Printf(color.RedString("%s\n", msg))
		os.Exit(1)
	}
}

// GetXtrabackupVersion Extract xtrabackup major.minor.patch-revision four-part version number
func GetXtrabackupVersion(cfg *config.Config) [4]int {
	// Resolve xtrabackup path
	xtrabackupPath, _, err := utils.ResolveXtrabackupPath(cfg.XtrabackupPath, false)
	if err != nil {
		return [4]int{0, 0, 0, 0}
	}

	cmd := exec.Command(xtrabackupPath, "--version")
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

// XtrabackupVersionGreaterOrEqual Compare major.minor.patch-revision four-part version number
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
