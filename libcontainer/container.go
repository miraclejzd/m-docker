package libcontainer

import (
	"fmt"
	"m-docker/libcontainer/cgroup"
	"m-docker/libcontainer/config"
	"m-docker/libcontainer/constant"
	"os"
	"os/exec"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

type Container struct {
	*config.Config
	CgroupManager cgroup.CgroupManager

	// 复用已经存在的容器环境
	Shared bool
}

// 创建容器对象
func NewContainer(conf *config.Config, shared bool) (*Container, error) {
	// 创建 cgroup 管理器
	cgroupManager, err := cgroup.NewCgroupManager(conf.Cgroup.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to create cgroup manager: %v", err)
	}

	return &Container{
		Config:        conf,
		CgroupManager: cgroupManager,
		Shared:        shared,
	}, nil
}

// 创建容器的运行环境
func (c *Container) Create() error {
	// 创建 rootfs
	if err := CreateRootfs(c.Config); err != nil {
		return fmt.Errorf("failed to create rootfs: %v", err)
	}

	// 挂载 volume
	if err := MountVolumes(c.Config); err != nil {
		return fmt.Errorf("failed to mount volumes: %v", err)
	}

	// 初始化 cgroup
	if err := c.CgroupManager.Init(); err != nil {
		return fmt.Errorf("failed to init cgroup: %v", err)
	}
	// 设置 cgroup 的资源限制
	c.CgroupManager.Set(c.Config.Cgroup.Resources)

	return nil
}

// 启动容器
func (c *Container) Start() error {
	// 生成一个容器进程的句柄，它启动后会运行 m-docker init [command]
	process, writePipe, err := c.newInitProcess()
	if err != nil {
		return fmt.Errorf("failed to create new process:  %v", err)
	}

	// 启动容器进程
	if err := process.Start(); err != nil {
		return fmt.Errorf("failed to run process.Start(): %v", err)
	}
	c.Config.Pid = process.Process.Pid
	c.Config.Status = constant.ContainerRunning

	// 将容器的配置信息持久化到磁盘上
	if err := config.RecordContainerConfig(c.Config); err != nil {
		return fmt.Errorf("failed to record container config: %v", err)
	}

	// 将容器进程加入到 cgroup 中
	if err := c.CgroupManager.Apply(c.Config.Pid); err != nil {
		return fmt.Errorf("failed to apply process %v to cgroup: %v", c.Config.Pid, err)
	}

	// 子进程创建之后再通过管道发送参数
	sendCommand(c.Config.CmdArray, writePipe)

	// 等待容器进程结束
	err = process.Wait()

	return nil
}

// 清理容器数据
func (c *Container) Remove() {
	log.Debugf("Remove container %s", c.Config.ID)
	// 删除容器的状态信息
	config.DeleteContainerState(c.Config)

	// 只有创建了环境的容器才需要进行的清理
	if !c.Shared {
		// 释放 cgroup
		c.CgroupManager.Destroy()

		// 卸载 volume
		UmountVolumes(c.Config)

		// 删除 rootfs
		DeleteRootfs(c.Config)
	}
}

// 生成一个容器进程的句柄
// 该容器进程将运行 m-docker init ，并视情况是否创建新的 UTS、PID、Mount、NET、IPC namespace
func (c *Container) newInitProcess() (*exec.Cmd, *os.File, error) {
	conf := c.Config

	// 创建一个匿名管道用于传递参数，readPipe 和 writePipe 分别传递给子进程和父进程
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		return nil, nil, fmt.Errorf("new pipe error: %v", err)
	}

	// 该进程会调用符号链接 /proc/self/exe，也就是 m-docker 这个可执行文件，并传递参数 init 和 [command]，即运行 m-docker init [command]
	cmd := exec.Command("/proc/self/exe", "init")

	// 如果不是复用已经存在的容器环境，就需要创建新的 UTS、PID、Mount、NET、IPC namespace
	if !c.Shared {
		// CLoneflags 参数表明这个句柄将以 clone 系统调用创建进程
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
				syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
		}
	}

	// 将 readPipe 通过子进程的 cmd.ExtraFile 传递给子进程
	cmd.ExtraFiles = []*os.File{readPipe}

	// 如果用户指定了 -it 参数，就需要把容器进程的输入输出导入到标准输入输出上
	if conf.TTY {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else { // 否则将容器进程的输出重定向到日志文件
		// 创建容器的状态信息目录
		if err := os.MkdirAll(conf.StateDir, 0777); err != nil {
			return nil, nil, fmt.Errorf("failed to create container state dir:  %v", err)
		}

		// 创建容器的日志文件
		logFile, err := os.Create(conf.LogPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create log file: %v", err)
		}
		// 这里一定不能关闭文件描述符，不然子进程无法访问，会导致日志文件无法写入
		//defer logFile.Close()

		// 将日志文件通过子进程的 cmd.ExtraFile 传递给子进程
		cmd.ExtraFiles = append(cmd.ExtraFiles, logFile)

		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	// 设置容器进程的工作目录为 UnionFS 联合挂载后所得到的 rootfs 目录
	cmd.Dir = conf.Rootfs

	// 设置容器进程的环境变量
	cmd.Env = append(os.Environ(), conf.Env...)

	return cmd, writePipe, nil
}

// 通过匿名管道发送参数给子进程
func sendCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	log.Debugf("Send command: %s", command)
	_, _ = writePipe.WriteString(command)
	_ = writePipe.Close()
}
