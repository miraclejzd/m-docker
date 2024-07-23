package cmd

import (
	"m-docker/libcontainer"
	"m-docker/libcontainer/cgroup"
	"m-docker/libcontainer/cgroup/resource"
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
		resConf := &resource.ResourceConfig{
			MemoryLimit: context.String("mem"),
			CpuLimit:    context.Float64("cpu"),
		}
		run(tty, cmdArray, resConf)
		return nil
	},
}

func run(tty bool, comArray []string, resConf *resource.ResourceConfig) {
	// 生成一个容器进程的句柄，它启动后会运行 m-docker init [command]
	process, writePipe := libcontainer.NewContainerProcess(tty)
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
	// 当前函数 return 后释放 cgroup
	defer cgroupManager.Destroy()
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
	sendInitCommand(comArray, writePipe)

	_ = process.Wait()
}

// 通过匿名管道发送参数给子进程
func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	log.Infof("Send command to init: %s", command)
	_, _ = writePipe.WriteString(command)
	_ = writePipe.Close()
}
