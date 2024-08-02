# chapter7 - 重构！添加容器生命周期

上一节我们为容器添加了 Config，走出了我们重构代码的第一步。

我们观察一下目前的 `cmd/run.go` 的代码，尤其是 `run` 函数：

```go
func run(conf *config.Config) {
	// 构建 rootfs
	if err := libcontainer.CreateRootfs(conf); err != nil {
		log.Errorf("Create rootfs error: %v", err)
		return
	}

	// 生成一个容器进程的句柄，它启动后会运行 m-docker init [command]
	process, writePipe := libcontainer.NewContainerProcess(conf)
	if process == nil {
		log.Errorf("New process error!")
		return
	}

	// 启动容器进程
	if err := process.Start(); err != nil {
		log.Errorf("Run process.Start() err: %v", err)
		return
	}

	conf.Pid = process.Process.Pid
	// 将容器的配置信息持久化到磁盘上
	if err := config.RecordContainerConfig(conf); err != nil {
		log.Errorf("Record container config error: %v", err)
		return
	}

	cgroupManager, err := cgroup.NewCgroupManager(conf.Cgroup.Path)
	// 当前进程结束后，释放资源
	defer func() {
		// 删除 rootfs
		libcontainer.DeleteRootfs(conf)

		// 当前函数 return 后释放 cgroup
		cgroupManager.Destroy()

		// 删除容器的状态信息
		config.DeleteContainerState(conf)
	}()

    // 省略...
}
```

迎面而来一股工业 shit 山的气息。

虽然函数封装的程度相当高了，但是看上去依旧显得很臃肿。最主要的原因：这个文件是子命令 `run` 的运行逻辑，我们在 `run` 的函数体内填满这么多的内容，似乎潜台词在说：这些是专属于 `run` 的执行逻辑。

实际上，我们从容器生命周期的角度去看，`run` 这个命令，其实可以等价为 `create`、`start` 这两个流程。创建 `rootfs`、`cgroup` 会被划分到 `create` 流程中，创建容器进程句柄并传递 command 参数会被划分到 `start` 流程中。

也就是说，我们需要对 `容器生命周期` 进行抽象，这样 `run` 命令以及之后可能会实现的 `create`、`start`、等命令可以很方便、直接地调用对应的流程函数，既增加代码的可读性，也让这些函数更内聚。

所以，这一节我们要对代码做进一步的重构，抽象出容器的生命周期。

## 1. 实现思路

容器的生命周期大致可分为这么几个流程：

- **create**：创建所需要的运行环境，例如 rootfs、cgroup。
- **start**：创建容器进程，并在运行环境下运行。
- **stop**：停止容器的运行，但保留运行环境。
- **remove**：删除运行环境。

针对 `run` 命令，它所涉及的流程为 **create**、**start**、**remove**，于是我们这一节实现这三个生命周期即可，**stop** 留到后面需要的时候一并实现。

## 2. 具体实现

### libcontainer/container.go

```go
// Container 对象，管理容器的生命周期
type Container struct {
	*config.Config
	CgroupManager cgroup.CgroupManager
}
```

我们定义一个 `Contanier` 对象负责容器生命周期的管理，由于 `Create` 方法需要创建 rootfs 与 cgroup，因此它至少需要拥有 `Config` 对象以及 `CgroupManager` 对象这两个属性。

我们需要封装一个 `NewContainer` 函数供外部调用，来生成 `Container` 对象。

```go
// 创建容器对象
func NewContainer(conf *config.Config) *Container {
	return &Container{
		Config: conf,
	}
}
```

接着我们要开始实现 `Create`、`Start`、`Remove` 这三个方法。

```go
// 创建容器的运行环境
func (c *Container) Create() error {
	// 创建 rootfs
	if err := CreateRootfs(c.Config); err != nil {
		return fmt.Errorf("failed to create rootfs: %v", err)
	}

	// 创建 cgroup Manager
	cgroupManager, err := cgroup.NewCgroupManager(c.Config.Cgroup.Path)
	if err != nil {
		return fmt.Errorf("failed to create cgroup manager: %v", err)
	}
	c.CgroupManager = cgroupManager
	// 初始化 cgroup
	if err = cgroupManager.Init(); err != nil {
		return fmt.Errorf("failed to init cgroup: %v", err)
	}
	// 设置 cgroup 的资源限制
	cgroupManager.Set(c.Config.Cgroup.Resources)

	return nil
}
```

参照 `cmd/run.go` 文件里的逻辑，目前我们的 `Create` 方法只需要创建 rootfs 与 cgroup 即可。

```go
// 启动容器
func (c *Container) Start() error {
	// 生成一个容器进程的句柄，它启动后会运行 m-docker init [command]
	process, writePipe, err := newContainerProcess(c.Config)
	if err != nil {
		return fmt.Errorf("failed to create new process:  %v", err)
	}

	// 启动容器进程
	if err := process.Start(); err != nil {
		return fmt.Errorf("failed to run process.Start(): %v", err)
	}
	c.Config.Pid = process.Process.Pid

	// 将容器的配置信息持久化到磁盘上
	if err := config.RecordContainerConfig(c.Config); err != nil {
		return fmt.Errorf("failed to record container config: %v", err)
	}

	// 将容器进程加入到 cgroup 中
	if err := c.CgroupManager.Apply(c.Config.Pid); err != nil {
		return fmt.Errorf("failed to apply process %v to cgroup: %v", c.Config.Pid, err)
	}

	// 子进程创建之后再通过管道发送参数
	sendInitCommand(c.Config.CmdArray, writePipe)

	return process.Wait()
}
```

`Start` 方法的任务则是创建容器进程，并让其在 `Create` 方法所创建好的运行环境下运行。目前我们只需要创建容器进程句柄后，将进程加入到 cgroup 中，让其运行即可（运行的同时还需保存状态信息至宿主机磁盘上）

```go
// 清理容器数据
func (c *Container) Remove() {
	// 删除 rootfs
	DeleteRootfs(c.Config)

	// 释放 cgroup
	c.CgroupManager.Destroy()

	// 删除容器的状态信息
	config.DeleteContainerState(c.Config)
}
```

`Remove` 方法则是清理容器数据。我们通过 `Create` 方法创建了 `rootfs`、`cgroup`，并在 `Start` 方法中保存了容器的状态信息，这里我们将它们都删除即可。

最后，我们将 `cmd/run.go` 中的代码进行重构：

```go
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
```

是不是相当的简洁易懂？

## 3. 测试

这次我们只要验证一下 `run` 命令是否正常运行即可。

我们创建一个新的容器：

```bash
# go build && sudo ./m-docker run -it
root@master58:/#
```

目前是正常的，我们打开一个新的终端，看看宿主机上容器的状态信息目录：

```bash
# sudo ls /var/run/m-docker/
a0b530fc16eceb98a0dd1ff783deabc8702d4936ce74ca5aaae6781e92263a31

# sudo cat /var/run/m-docker/a0b530fc16eceb98a0dd1ff783deabc8702d4936ce74ca5aaae6781e92263a31/config.json
{"status":"","pid":1016655,"ID":"a0b530fc16eceb98a0dd1ff783deabc8702d4936ce74ca5aaae6781e92263a31","name":"competent_bassi","rootfs":"/var/lib/m-docker/rootfs/a0b530fc16eceb98a0dd1ff783deabc8702d4936ce74ca5aaae6781e92263a31","rwLayer":"/var/lib/m-docker/layers/a0b530fc16eceb98a0dd1ff783deabc8702d4936ce74ca5aaae6781e92263a31","stateDir":"/run/m-docker/a0b530fc16eceb98a0dd1ff783deabc8702d4936ce74ca5aaae6781e92263a31","tty":true,"CmdArray":["/bin/bash"],"cgroup":{"name":"m-docker-a0b530fc16eceb98a0dd1ff783deabc8702d4936ce74ca5aaae6781e92263a31","path":"/sys/fs/cgroup/m-docker.slice/m-docker-a0b530fc16eceb98a0dd1ff783deabc8702d4936ce74ca5aaae6781e92263a31.scope","Memory":"max","cpuPeriod":100000,"cpuQuota":0},"createdTime":"20232+00-100 00:28:78"}
```

可以看到容器的状态信息已经保存到了磁盘上。

至此，我们可以初步判定这次重构是成功的。