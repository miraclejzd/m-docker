package libcontainer

import (
	"os"
	"os/exec"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// 生成一个容器进程的句柄
// 该容器进程将运行 m-docker init ，并拥有新的 UTS、PID、Mount、NET、IPC namespace
func NewContainerProcess(tty bool) (*exec.Cmd, *os.File) {
	// 创建一个匿名管道用于传递参数，readPipe 和 writePipe 分别传递给子进程和父进程
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		log.Errorf("New pipe error: %v", err)
		return nil, nil
	}

	// 该进程会调用符号链接 /proc/self/exe，也就是 m-docker 这个可执行文件，并传递参数 init 和 [command]，即运行 m-docker init [command]
	cmd := exec.Command("/proc/self/exe", "init")

	// CLoneflags 参数表明这个句柄将以 clone 系统调用创建进程，并设置了新的 UTS、PID、Mount、NET 和 IPC namespace
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}

	// 如果用户指定了 -it 参数，就需要把容器进程的输入输出导入到标准输入输出上
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	// 将 readPipe 通过子进程的 cmd.ExtraFile 传递给子进程
	cmd.ExtraFiles = []*os.File{readPipe}

	// 设置容器进程的工作目录为 UnionFS 联合挂载后所得到的 rootfs 目录
	cmd.Dir = "/var/lib/m-docker/rootfs/default"

	return cmd, writePipe
}
