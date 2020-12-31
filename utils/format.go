package utils

import (
	"fmt"
	tm "github.com/buger/goterm"
	"github.com/fatih/color"
	"strings"
	"unicode/utf8"
)

func output(item, msg, suggest string, ok bool) {
	tip := color.GreenString("[DONE]")
	if !ok {
		tip = color.RedString("[ERROR]")
	}
	msg = padding(msg, 60)
	suggest= padding(suggest, 20)
	var Output = tm.NewTable(0, 4, 2, ' ', 0)
	fmt.Fprintf(Output, "\t%s\t%s\t%s\t%s", item, msg, suggest, tip)
	tm.Println(Output)
	tm.Flush()
}

func padding(item string, length int) string {
	itemLen := utf8.RuneCountInString(item)
	if itemLen < length {
		item += strings.Repeat(" ", length - itemLen)
	}
	return item
}
