package client

import (
	"fmt"
	"os"

	flag "github.com/docker/docker/pkg/mflag"
)

// CmdHelp displays information on a Docker command.
//
// If more than one command is specified, information is only shown for the first command.
//
// Usage: docker help COMMAND or docker COMMAND --help
func (cli *DockerCli) CmdHelp(args ...string) error {
	fmt.Println("args of help:", args)
	if len(args) > 1 {
		method, exists := cli.getMethod(args[:2]...)
		if exists {
			method("--help")
			return nil
		}
	}
	// 如果只给出了一个命令  ps
	if len(args) > 0 {
		// 那么得到ps命令对应的函数
		method, exists := cli.getMethod(args[0])
		// 如果不存在,那么给用户提示命令不存在
		if !exists {
			fmt.Fprintf(cli.err, "docker: '%s' is not a docker command. See 'docker --help'.\n", args[0])
			os.Exit(1)
		} else {
			// 如果存在,使用--help来调用这个命令
			method("--help")
			return nil
		}
	}

	// 如果没有指明要显示哪个命令的帮助,那么输出usage
	flag.Usage()

	return nil
}
