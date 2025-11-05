package format

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tm "github.com/buger/goterm"
	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
)

// Output formats and prints a table row with status indicator
func Output(item, msg, suggest string, ok bool) {
	tip := color.GreenString("[DONE]")
	if !ok {
		tip = color.RedString("[ERROR]")
	}
	msg = padding(msg, 60)
	suggest = padding(suggest, 20)
	var table = tm.NewTable(0, 4, 2, ' ', 0)
	fmt.Fprint(table, i18n.Sprintf("\t%s\t%s\t%s\t%s", item, msg, suggest, tip))
	tm.Println(table)
	tm.Flush()
}

// padding adds spaces to ensure consistent column width
func padding(item string, length int) string {
	itemLen := utf8.RuneCountInString(item)
	if itemLen < length {
		item += strings.Repeat(" ", length-itemLen)
	}
	return item
}
