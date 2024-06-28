package cmd

import (
	"fmt"
	"m-docker/libcontainer"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// m-docker run 命令
var RunCommand = cli.Command{
	Name:      "run",
	Usage:     `create and run a container`,
	UsageText: `m-docker run -it [command]`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "it", // 简单起见，这里把 -i 和 -t 合并了
			Usage: "enable tty",
		},
	},

	// m-docker run 命令的入口点
	// 1. 判断参数是否含有 command
	// 2. 获取 command
	// 3. 调用 run 函数去创建和运行容器
	Action: func(context *cli.Context) error {
		if context.NArg() < 1 {
			return fmt.Errorf("missing container command")
		}
		cmd := context.Args().Get(0)
		tty := context.Bool("it")
		run(tty, cmd)
		return nil
	},
}

func run(tty bool, command string) {
	// 生成一个容器进程的句柄，它启动后会运行 m-docker init [command]
	process := libcontainer.NewContainerProcess(tty, command)

	// 启动容器进程
	if err := process.Start(); err != nil {
		log.Error(err)
	}
	_ = process.Wait()
	os.Exit(-1)
}
