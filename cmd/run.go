package cmd

import (
	"m-docker/libcontainer"
	"m-docker/libcontainer/cgroup"
	"m-docker/libcontainer/cgroup/resource"
	"m-docker/libcontainer/config"
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
		config := config.CreateConfig(context)

		resConf := &resource.ResourceConfig{
			MemoryLimit: context.String("mem"),
			CpuLimit:    context.Float64("cpu"),
		}

		run(config, resConf)
		return nil
	},
}

func run(config *config.Config, resConf *resource.ResourceConfig) {
	// 构建 rootfs
	if err := libcontainer.CreateRootfs(); err != nil {
		log.Errorf("Create rootfs error: %v", err)
		return
	}

	// 生成一个容器进程的句柄，它启动后会运行 m-docker init [command]
	process, writePipe := libcontainer.NewContainerProcess(config.TTY)
	if process == nil {
		log.Errorf("New process error!")
		return
	}

	// 启动容器进程
	if err := process.Start(); err != nil {
		log.Errorf("Run process.Start() err: %v", err)
		return
	}

	cgroupManager, err := cgroup.NewCgroupManager("m-docker.slice")
	// 当前进程结束后，释放资源
	defer func() {
		// 删除 rootfs
		libcontainer.DeleteRootfs()

		// 当前函数 return 后释放 cgroup
		cgroupManager.Destroy()
	}()
	if err != nil {
		log.Errorf("Create new cgroup manager fail: %v", err)
		return
	}
	// 初始化 cgroup
	if err = cgroupManager.Init(); err != nil {
		log.Errorf("Init cgroup fail: %v", err)
		return
	}
	// 将子进程加入到 cgroup 中
	if err = cgroupManager.Apply(process.Process.Pid); err != nil {
		log.Errorf("Apply process %v to cgroup fail: %v", process.Process.Pid, err)
		return
	}
	// 设置 cgroup 的资源限制
	cgroupManager.Set(resConf)

	// 子进程创建之后再通过管道发送参数
	sendInitCommand(config.CmdArray, writePipe)

	_ = process.Wait()
}

// 通过匿名管道发送参数给子进程
func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	log.Infof("Send command to init: %s", command)
	_, _ = writePipe.WriteString(command)
	_ = writePipe.Close()
}
