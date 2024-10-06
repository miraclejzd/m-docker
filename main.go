package main

import (
	"os"

	"github.com/urfave/cli"

	log "github.com/sirupsen/logrus"

	"m-docker/cmd"
)

const (
	usage = `a simple container runtime implementation.

The purpose of this project is to learn how docker(exactly, runC) works and how to write a docker by ourselves.
Enjoy it, just for fun.`
)

// main 函数是整个程序的入口
// 使用的是 github.com/urfave/cli 框架来构建命令行工具
func main() {
	app := cli.NewApp()
	app.Name = "m-docker"
	app.Usage = usage

	// 添加 run 等子命令
	app.Commands = []cli.Command{
		cmd.RunCommand,
		cmd.InitCommand,
		cmd.ContainerListCommand,
		cmd.LogsCommand,
		cmd.ExecCommand,
	}
	// 全局 flag
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug", // 启用 debug 模式
			Usage: "enable debug mode",
		},
	}
	app.Before = func(context *cli.Context) error {
		// 设置日志格式
		log.SetFormatter(&log.TextFormatter{
			ForceColors:   true,
			FullTimestamp: true,
		})
		// 设置日志级别
		if context.Bool("debug") {
			log.SetLevel(log.DebugLevel)
		}

		log.SetOutput(os.Stdout)
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
