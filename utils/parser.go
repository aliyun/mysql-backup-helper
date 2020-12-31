package utils

import (
	"fmt"
	"github.com/alyu/configparser"
	"strings"
)

const MYSQLD = "mysqld"

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func Parser(filename string) map[string]string {
	fmt.Println("解析文件:", filename)
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
