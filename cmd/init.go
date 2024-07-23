package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		log.Infof("--- Inside the container ---")
		err := initContainer()
		return err
	},
}

// 在容器中进行初始化
// 执行到这里的时候容器已经被创建，所以这个函数是在容器内部执行的
func initContainer() error {
	log.Infof("Start func: initContainer")

	// 挂载根文件系统
	mountRootFS()

	// 读取管道中的 command 参数
	cmdArray := readPipeCommand()
	if len(cmdArray) == 0 {
		return errors.New("get user command error, cmdArray is nil")
	}

	// 判断用户指定的 command 的可执行文件路径是否存在
	path, err := exec.LookPath(cmdArray[0])
	if err != nil {
		log.Errorf("exec.LookPath error: %v", err)
		return err
	}
	log.Infof("find command path: %s", path)

	// syscall.Exec 会调用 execve 系统调用，它会用新的程序段替换当前进程的程序段
	// 成功执行这个系统调用后，当前 initContainer 函数剩余的程序段将不会继续运行，而是被用户定义的 command 替换
	// 如果失败了才会返回错误，继续执行剩下的程序段
	if err := syscall.Exec(path, cmdArray, os.Environ()); err != nil {
		log.Errorf("syscall.Exec error: %v", err.Error())
	}

	return nil
}

// 挂载根文件系统
func mountRootFS() {
	pwd, err := os.Getwd()
	if err != nil {
		log.Errorf("Get cwd error: %v", err)
		return
	}
	log.Infof("Current working directory: %s", pwd)

	// 实现 mount --make-rprivate /
	// 使得容器内的根挂载点与宿主机的根挂载点隔离开来
	_ = syscall.Mount("none", "/", "none", syscall.MS_PRIVATE|syscall.MS_REC, "")

	err = pivotRoot(pwd)
	if err != nil {
		log.Errorf("pivotRoot error: %v", err)
		return
	}

	// 通过 mount 挂载容器自己的 proc 文件系统
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	_ = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
}

// 调用 pivot_root 系统调用，将根文件系统设置为 newRoot
// pivot_root 系统调用原型：
// int pivot_root(const char *new_root, const char *put_old);
func pivotRoot(newRoot string) error {
	// pivot_root 系统调用要求 new_root 和 put_old 都是挂载点
	// 考虑到 newRoot 可能并不是挂载点，因此使用 bind mount 将其转化为挂载点
	if err := syscall.Mount(newRoot, newRoot, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mount rootfs to itself error: %v", err)
	}

	// 创建 root/.put_old 目录，用于存放旧的 rootFS
	putOld := filepath.Join(newRoot, ".put_old")
	if err := os.Mkdir(putOld, 0700); err != nil {
		return fmt.Errorf("create dir %s error: %v", putOld, err)
	}

	// 执行 pivot_root 系统调用
	if err := syscall.PivotRoot(newRoot, putOld); err != nil {
		return fmt.Errorf("syscall.PivotRoot(%s, %s) error: %v", newRoot, putOld, err)
	}

	// 切换到新的根目录
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("change dir to / error: %v", err)
	}

	// umount 旧的 rootFS
	// 由于切换了根目录，putOld 路径变成了 /.put_old
	putOld = filepath.Join("/", ".put_old")
	if err := syscall.Unmount(putOld, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("umount old rootfs error: %v", err)
	}

	// 删除 putOld 临时目录
	return os.Remove(putOld)
}

const readPipefdIndex = 3

func readPipeCommand() []string {
	// uintPtr(3) 就是指 index 为 3 的文件描述符，至于为什么是3，具体解释一下：
	// 每个进程在创建的时候默认有3个文件描述符，分别是：
	// 0: 标准输入
	// 1: 标准输出
	// 2: 标准错误
	// 我们在之前创建 cmd 时设置了 cmd.ExtraFiles = []*os.File{readPipe}
	// 因此这里的 index 就是3
	pipe := os.NewFile(uintptr(readPipefdIndex), "pipe")

	msg, err := io.ReadAll(pipe)
	if err != nil {
		log.Errorf("read pipe error: %v", err)
		return nil
	}

	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}
