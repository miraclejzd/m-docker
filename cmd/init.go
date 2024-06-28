package cmd

import (
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// m-docker init 命令（它不可以被显式调用）
var InitCommand = cli.Command{
	Name:   "init",
	Usage:  `Init the container process, do not call it outside!`,
	Hidden: true, // 隐藏该命令，避免被显式调用

	// 1. 获取传递来的 command 参数
	// 2. 在容器中进行初始化
	Action: func(context *cli.Context) error {
		log.Infof("Inside the container!")
		command := context.Args().Get(0)
		log.Infof("command: %s", command)
		err := initContainer(command)
		return err
	},
}

func initContainer(command string) error {
	log.Infof("Start func: initContainer")

	// 实现 mount --make-rprivate /proc
	// 使得容器内的 /proc 目录与宿主机的 /proc 目录隔离开来
	flags := uintptr(syscall.MS_PRIVATE | syscall.MS_REC)
	_ = syscall.Mount("none", "/proc", "none", flags, "")

	// 挂载容器自己的 proc 文件系统
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	_ = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	// syscall.Exec 会调用 execve 系统调用，它会用新的程序段替换当前进程的程序段
	// 成功执行这个系统调用后，当前 initContainer 函数剩余的程序段将不会继续运行，而是被用户定义的 command 替换
	// 如果失败了才会返回错误，继续执行剩下的程序段
	argv := []string{command}
	if err := syscall.Exec(command, argv, os.Environ()); err != nil {
		log.Errorf(err.Error())
	}

	return nil
}
