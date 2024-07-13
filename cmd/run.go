package cmd

import (
	"m-docker/libcontainer"
	"os"
	"strings"

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
		var cmdArray []string
		if context.NArg() < 1 {
			log.Warnf("missing container command, filling with '/bin/bash' ")
			cmdArray = append(cmdArray, string("/bin/bash"))
		} else {
			for _, arg := range context.Args() {
				cmdArray = append(cmdArray, arg)
			}
		}
		tty := context.Bool("it")
		run(tty, cmdArray)
		return nil
	},
}

func run(tty bool, comArray []string) {
	// 生成一个容器进程的句柄，它启动后会运行 m-docker init [command]
	process, writePipe := libcontainer.NewContainerProcess(tty)
	if process == nil {
		log.Errorf("New process error!")
		return
	}

	// 启动容器进程
	if err := process.Start(); err != nil {
		log.Errorf("Run process.Start() err: %v", err)
	}
	// 子进程创建之后再通过管道发送参数
	sendInitCommand(comArray, writePipe)

	_ = process.Wait()
	os.Exit(-1)
}

// 通过匿名管道发送参数给子进程
func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	log.Infof("Send command to init: %s", command)
	_, _ = writePipe.WriteString(command)
	_ = writePipe.Close()
}
