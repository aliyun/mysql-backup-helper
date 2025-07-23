package utils

import (
	"strings"

	"github.com/alyu/configparser"
	"github.com/gioco-play/easy-i18n/i18n"
)

const MYSQLD = "mysqld"

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func Parser(filename string) map[string]string {
	i18n.Printf("Parsing file: %s\n", filename)
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
