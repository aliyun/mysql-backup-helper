package main

import (
	"backup-helper/utils"
	"flag"
	"fmt"
	"golang.org/x/term"
	"os"
	"strings"
)

type CmdOpt struct {
	host string
	port int
	user string
	password string
}

func main() {
	opt := parseCmd()
	fmt.Println(fmt.Sprintf("连接数据库host=%s port=%d user=%s", opt.host, opt.port, opt.user))
	outputHeader()
	db := utils.GetConnection(opt.host, opt.port, opt.user, opt.password)
	defer db.Close()
	options := utils.CollectVariableFromMySQLServer(db)
	utils.Check(options)
}

func parseCmd() CmdOpt {
	var host, user, password string
	var port int
	flag.StringVar(&host, "host", "127.0.0.1", "Connect to host")
	flag.IntVar(&port, "port", 3306, "Port number to use for connection")
	flag.StringVar(&user, "user", "root", "User for login")
	flag.StringVar(&password, "password", "", "Password to use when connecting to server. If password is not given it's asked from the tty.")
	flag.Parse()

	opts := CmdOpt {host, port, user, password}

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(0)
	}

	if "" == password {
		fmt.Printf("请输入数据库密码: ")
		pwd, _ := term.ReadPassword(0)
		fmt.Println()
		opts.password = string(pwd)
	}
	return opts
}

func outputHeader() {
	bar := strings.Repeat("#", 120)
	fmt.Println(bar)
	fmt.Println("  MySQL backup pre-check")
	fmt.Println("  2020-2020 Alibaba Cloud Inc")
	fmt.Println(bar)
}
