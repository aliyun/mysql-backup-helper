package utils

import (
	"github.com/alyu/configparser"
	"github.com/mylukin/easy-i18n/i18n"
	"strings"
)

const MYSQLD = "mysqld"

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func Parser(filename string) map[string]string {
	i18n.Printf("解析文件:", filename)
	configparser.Delimiter = "="
	config, err := configparser.Read(filename)
	check(err)
	section, err := config.Section(MYSQLD)
	if err != nil {
		section, err = config.Section(strings.ToUpper(MYSQLD))
	}
	check(err)
	options := section.Options()
	return options
}
