package cmd

import (
	"m-docker/libcontainer"
	"m-docker/libcontainer/config"

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
		cli.StringFlag{
			Name:  "mem", // 内存限制
			Usage: "memory limit.   eg: -mem 100m",
		},
		cli.StringFlag{
			Name:  "cpu", // CPU 使用率限制
			Usage: "cpu limit.    eg: -cpu 0.5",
		},
		cli.StringFlag{
			Name:  "name", // 容器名称
			Usage: "container name.	eg: -name my-ubuntu-env",
		},
	},

	// m-docker run 命令的入口点
	// 1. 判断参数是否含有 command
	// 2. 获取 command
	// 3. 调用 run 函数去创建和运行容器
	Action: func(context *cli.Context) error {
		// 生成容器的配置信息
		conf := config.CreateConfig(context)

		run(conf)

		return nil
	},
}

func run(conf *config.Config) {
	// 创建容器对象
	container := libcontainer.NewContainer(conf)
	// 函数结束后释放容器资源
	defer container.Remove()

	// 创建容器运行环境
	if err := container.Create(); err != nil {
		log.Errorf("Create container error: %v", err)
		return
	}

	// 启动容器
	if err := container.Start(); err != nil {
		log.Errorf("Start container error: %v", err)
		return
	}
}
