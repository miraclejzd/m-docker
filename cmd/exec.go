package cmd

import (
	"fmt"
	"m-docker/libcontainer"
	"m-docker/libcontainer/config"
	"m-docker/libcontainer/constant"
	"path"
	"strconv"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var ExecCommand = cli.Command{
	Name:      "exec",
	Usage:     `exec the command inside a container`,
	UsageText: `m-docker exec [OPTIONS] CONTAINER COMMAND`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "it", // 简单起见，这里把 -i 和 -t 合并了
			Usage: "enable tty",
		},
		cli.BoolFlag{
			Name:  "d",
			Usage: "detach mode",
		},
	},

	Action: func(context *cli.Context) error {
		// 参数校验
		if err := validateExecArgs(context); err != nil {
			return fmt.Errorf("validate args failed: %v", err)
		}

		// 生成容器 Config
		conf, err := getExecConfig(context)
		if err != nil {
			return fmt.Errorf("get Config failed: %v", err)
		}

		// 若前台运行，则当前这个进程之间管理容器生命周期
		// 启动容器进程后，当前进程会阻塞，等待容器运行结束
		if conf.TTY {
			execContainer(conf)
		} else { // 后台运行
			// fork 一个进程作为 shim 来管理容器生命周期
			// 之后当前这个 m-docker run 进程就可以退出了
			pid, _, errno := syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
			if errno != 0 {
				return fmt.Errorf("fork error: %v", err)
			}

			// 子进程
			if pid == 0 {
				log.Debugf("[shim process] fork success")
				return execContainer(conf)
			} else { // 父进程
				log.Debugf("[father process] fork shim process, pid: %d", pid)
			}
		}

		return nil
	},
}

// 校验 exec 命令的参数
func validateExecArgs(ctx *cli.Context) error {
	if ctx.NArg() < 2 {
		return fmt.Errorf("missing container id and command")
	}
	return nil
}

// 生成 exec 命令的 Config
func getExecConfig(ctx *cli.Context) (*config.Config, error) {
	prefixOrName := ctx.Args().First()
	id, err := config.GetIDFromNameOrPrefix(prefixOrName)
	if err != nil {
		return nil, fmt.Errorf("failed to get container id: %v", err)
	}

	// 获取容器 Config
	conf, err := config.GetConfigFromID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get container config: %v", err)
	}

	// 修改 Config
	conf.Status = ""
	conf.TTY = ctx.Bool("it")
	// 获取 command 参数
	var cmdArray []string
	for i := 1; i < ctx.NArg(); i++ {
		cmdArray = append(cmdArray, ctx.Args().Get(i))
	}
	conf.CmdArray = cmdArray
	// 将状态信息持久化到 /tmp/m-docker/[id] 目录下
	conf.StateDir = path.Join(constant.TmpPath, conf.ID)
	conf.LogPath = path.Join(conf.StateDir, constant.LogFileName)
	// 传递环境变量
	conf.Env = append(conf.Env, fmt.Sprintf("%s=%s", constant.ENV_SETNS_PID, strconv.Itoa(conf.Pid)))
	conf.Env = append(conf.Env, fmt.Sprintf("%s=%s", constant.ENV_NOT_MOUNT_ROOTFS, "TRUE"))

	return conf, nil
}

// 执行 m-docker exec 命令
func execContainer(conf *config.Config) error {
	// 重新生成容器对象
	// 设置 shared 为 true，表示不创建新的环境
	container, err := libcontainer.NewContainer(conf, true)
	if err != nil {
		return fmt.Errorf("failed to create container object: %v", err)
	}
	// 结束后只需要删除状态信息即可，不能调用 Container.Remove()
	defer config.DeleteContainerState(conf)

	// 这里不调用 container.Create() 就不会创建新的环境
	// rootfs、statedir 等都是已经存在的
	if err := container.Start(); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}
	return nil
}
