package cmd

import (
	"fmt"
	"m-docker/libcontainer"
	"m-docker/libcontainer/config"
	"syscall"

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
		cli.BoolFlag{
			Name:  "d, detach", // 后台运行
			Usage: "detach container",
		},
		cli.StringFlag{
			Name:  "mem", // 内存限制
			Usage: "memory limit.	eg: -mem 100m",
		},
		cli.StringFlag{
			Name:  "cpu", // CPU 使用率限制
			Usage: "cpu limit.	eg: -cpu 0.5",
		},
		cli.StringFlag{
			Name:  "name", // 容器名称
			Usage: "container name.	eg: -name my-ubuntu-env",
		},
		cli.StringSliceFlag{
			Name:  "v", // 挂载目录
			Usage: "bind mount a volume.	eg: -v /host:/container",
		},
	},

	// m-docker run 命令的入口点
	// 1. 判断参数是否含有 command
	// 2. 获取 command
	// 3. 调用 run 函数去创建和运行容器
	Action: func(context *cli.Context) error {
		// 生成容器的配置信息
		conf, err := config.CreateConfig(context)
		if err != nil {
			return fmt.Errorf("create config error: %v", err)
		}

		// 若为前台运行，则由当前这个 m-docker run 进程直接管理容器生命周期
		// 启动容器进程后，当前进程会阻塞，等待容器运行结束
		if conf.TTY {
			run(conf)
		} else { // 后台运行
			// 打印容器 ID
			fmt.Printf("%v\n", conf.ID)

			// fork 一个进程作为 shim 来管理容器生命周期
			// 之后当前这个 m-docker run 进程就可以退出了
			pid, _, errno := syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
			if errno != 0 {
				return fmt.Errorf("fork error: %v", err)
			}

			// 子进程
			if pid == 0 {
				log.Debugf("[shim process] fork success")
				run(conf)
			} else { // 父进程
				log.Debugf("[father process] fork shim process, pid: %d", pid)
			}
		}

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
