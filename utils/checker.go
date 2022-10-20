package utils

import (
	"fmt"
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

var flag = true

func Check(options map[string]string) {
	checkVersion(options["version"])
	checkBackup(options)
	checkReplication(options)
	if flag {
		cmd := "\tinnobackupex --backup --host=<host> --port=<port> --user=<dbuser> --password=<password> --stream=xbstream --compress /home/mysql/backup  > /home/mysql/backup/backup_qp.xb"
		fmt.Println()
		i18n.Printf("备份命令参考(Percona XtraBackup):")
		fmt.Println()
		fmt.Println(cmd)
	}
}

func checkVersion(value string) {
	i18n.Printf("检查MySQL版本...")
	fmt.Println()
	v := getVersion(value)
	checkItem := i18n.Sprintf("版本")
	if v.major == 5 && v.minor == 7 && v.micro <= 32 {
		output(checkItem, value, "", true)
	} else if v.major == 8 && v.minor == 0 && v.micro <= 18 {
		output(checkItem, value, "", true)
	} else {
		output(checkItem, value, i18n.Sprintf("可能无法兼容"), false)
		fmt.Printf(color.HiWhiteString(i18n.Sprintf("\t您需要通过物理备份迁移到云上的数据库小版本较高，云上MySQL可能无法兼容该版本的数据文件，可在MySQL全量备份上云帮助文档页面确认")))
	}
	fmt.Println()
}

func checkInnodbFilePath(options map[string]string) {
	// file_name:file_size[:autoextend[:max:max_file_size]]
	key := "innodb_data_file_path"
	val := options[key]
	tokens := strings.Split(val, ";")
	checkValue := key + "=" + val
	if len(tokens) > 1 {
		output(i18n.Sprintf("参数"), checkValue, i18n.Sprintf("不支持多参数"), false)
		flag = false
	} else {
		filename := strings.Split(tokens[0], ":")[0]
		if filename == "ibdata1" {
			output(i18n.Sprintf("参数"), checkValue, "", true)
		} else {
			output(i18n.Sprintf("参数"), checkValue, i18n.Sprintf("建议参数: ibdata1"), false)
			flag = false
		}
	}
}

func checkBackup(options map[string]string) {
	i18n.Printf("检查备份相关参数...")
	fmt.Println()
	checkInnodbFilePath(options)
	i18n.Printf("备份相关参数完毕...")
	fmt.Println()
}

func checkReplication(options map[string]string) {
	i18n.Printf("检查复制参数中(以下参数影响主备复制, 并不影响备份)...")
	fmt.Println()

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
				suggest := i18n.Sprintf("建议参数: %s", item.value)
				fmt.Println()
				output(i18n.Sprintf("参数"), checkValue, suggest, false)
			} else {
				output(i18n.Sprintf("参数"), checkValue, "", true)
			}
		}
	}

	i18n.Printf("复制参数检查完毕")
	fmt.Println()
}

func checkMissVariable(key, value string) {
	checkValue := key + "=" + value
	if value == "0" {
		output(i18n.Sprintf("参数"), checkValue, i18n.Sprintf("参数未设置"), false)
	} else {
		output(i18n.Sprintf("参数"), checkValue, "", true)
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
