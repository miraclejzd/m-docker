# chapter3 - 基于cgroup实现资源限制

我们现在已经可以通过命令行 `m-docker run` 来创建容器了，但是这个容器是没有任何资源限制的，也就是说，容器可以无限制地使用宿主机的资源，这显然是不合理的。

这一节中，我们将使用 cgroup 对容器进行 cpu 和内存的资源限制，并添加 `-cpu` 和 `-mem` 参数，使用方法如下：

```bash
./m-docker run -it -cpu 0.5 -mem 100m /bin/bash
```

这样会限制容器的 cpu 使用率为 50%，内存为 100MB。

## 具体实现

首先为 `m-docker run` 命令添加 `-cpu` 和 `-mem` 的 flag。

### cmd/run.go

```go
var RunCommand = cli.Command{
	Name:      "run",
	Usage:     `create and run a container`,
	UsageText: `m-docker run -it [command]`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "it", 
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
        // 从命令行参数中解析出内存和 CPU 使用率限制
        // ResourceConfig 结构体保存 cgroup 信息，其定义后续会提到
		resConf := &resource.ResourceConfig{
			MemoryLimit: context.String("mem"),
			CpuLimit:    context.Float64("cpu"),
		}
		run(tty, cmdArray, resConf)
		return nil
	},
}
```

从命令行参数中解析出 `-cpu` 和 `-mem` 的参数，会被封装成 `ResourceConfig` 对象。

### libcontainer/cgroup/resrouce/resource.go

`ResourceConfig` 对象保存了 cgroup 的资源限制信息，当前仅包括内存限制和 CPU 使用率限制。

```go
type ResourceConfig struct {
	MemoryLimit string  // 内存限制,  e.g.  100m、100k
	CpuLimit    float64 // CPU 使用率限制,   e.g.  0.3、2.0
}
```

接下来我们需要实现各个 cgroup controller，首先我们需要定义一个 cgroup controller 的抽象接口。

### libcontainer/cgroup/v2/controller.go

```go
// cgroup controller 的抽象接口
type Controller interface {
	// Name() 方法返回当前 cgroup controller 的名字，如 cpu、memory
	Name() string

	// Set() 方法用于设置当前 cgroup controller 的资源限制
	Set(cgroupPath string, resConf *resource.ResourceConfig) error
}
```

`Controller` 接口是对 cgroup controller 的抽象，它要求 cgroup controller 实现这2个基本方法：

- `Name()` 返回当前 cgroup controller 的名字，如 cpu、memory
- `Set()` 根据 ResourceConfig，设置 cgroup 路径为 cgroupPath 下的 cgroup controller 的资源限制

```go
// 所有的 cgroup controller
var Controllers = []Controller{
	&CpuController{},
	&MemoryController{},
}
```

`Controllers` 则是所有实现了 `Controller` 接口的 cgroup controller 的集合。我们当前只打算进行 cpu 和内存的控制，因此只需要实现 cpu 和 内存的 cgroup controller 即可。

下面以 `CpuController` 为例。

### libcontainer/cgroup/v2/cpu.go

```go
// CpuController 类是对 Controller 接口的实现
type CpuController struct {
}

const (
	DefaultPeriod = 100000 // 默认调度周期为 100000us
)

func (s *CpuController) Name() string {
	return "cpu"
}

func (s *CpuController) Set(cgroupPath string, resConf *resource.ResourceConfig) error {
	var cpuLimit string
	if resConf.CpuLimit == 0 { // 如果没有设置 CPU 使用率限制，则默认为最大值
		cpuLimit = "max " + strconv.Itoa(DefaultPeriod)
	} else { // 如果设置了 CPU 使用率限制，则按照设置的值进行限制
		cpuLimit = fmt.Sprintf("%s %v", strconv.Itoa(int(DefaultPeriod*resConf.CpuLimit)), DefaultPeriod)
	}

	// 将 CPU 使用率限制写入 cpu.max 文件
	if err := os.WriteFile(path.Join(cgroupPath, "cpu.max"), []byte(cpuLimit), 0644); err != nil {
		return fmt.Errorf("os.WriteFile() to file %v fail:  %v", path.Join(cgroupPath, "cpu.max"), err)
	}

	log.Infof("Set cgroup cpu.max: %v", cpuLimit)
	return nil
}
```

可以看得出来，`Set` 方法只是把对 `cpu.max` 文件的写操作进行了封装，这与我们直接操控命令行去修改 cgroup 文件系统是一样的：

- 在 cgroupPath 下找到 `cpu.max` 文件
- 将 cpu 的资源限制写入 `cpu.max` 文件

另外的 `MemoryController` 也是类似的，这里就不贴代码了。

在实现了各个 cgroup controller 之后，我们便可以开始抽象 cgroup 本身了（毕竟 cgroup 就是由多个 cgroup controller 组成的一个集合罢了）。

### libcontainer/cgroup/cgroup.go

```go
// Cgroup 是 cgroup 的抽象接口
type Cgroup interface {
	// 初始化 cgroup，创建 cgroup 目录
	Init() error

	// 将进程 pid 添加至 cgroup 中
	Apply(pid int) error

	// 设置 cgroup 的资源限制
	Set(res *resource.ResourceConfig)

	// 销毁 cgroup
	Destroy()
}
```

`Cgroup` 接口是对 cgroup 的抽象，它要求 cgroup 实现这4个基本方法：
- `Init()`：初始化，在宿主机上创建 cgroup 目录
- `Apply()`：将进程 pid 添加至 cgroup.procs 中
- `Set()`：设置 cgroup 的资源限制
- `Destroy()`：销毁 cgroup，删除 cgroup 目录

目前主流的 cgroup 版本是 cgroup v2，这里我们需要实现对 cgroup v2 的支持。

### libcontainer/cgroup/v2/manager.go

```go
type CgroupV2Manager struct {
	dirPath     string
	resource    *resource.ResourceConfig
	controllers []Controller
}

func (c *CgroupV2Manager) Init() error {
	_, err := os.Stat(c.dirPath)
	if err != nil && os.IsNotExist(err) { // 如果 cgroup 目录不存在，则创建
		err := os.Mkdir(c.dirPath, 0755)
		if err != nil {
			return fmt.Errorf("create cgroup dir \"%v\" fail: %v", c.dirPath, err)
		}
	} else { // 如果 cgroup 目录已经存在，则返回错误
		return fmt.Errorf("cgroup dir %s already exists", c.dirPath)
	}
	return nil
}

func (c *CgroupV2Manager) Apply(pid int) error {
	// 将进程的 PID 写入 cgroup.procs 文件
	if err := os.WriteFile(path.Join(c.dirPath, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("os.WriteFile() to file %v fail: %v", path.Join(c.dirPath, "cgroup.procs"), err)
	}

	return nil
}

func (c *CgroupV2Manager) Set(resConf *resource.ResourceConfig) {
	c.resource = resConf
	// 遍历所有的 cgroup controller，调用 controller 的 Set 方法来设置 cgroup 的资源限制
	for _, controller := range c.controllers {
		if err := controller.Set(c.dirPath, resConf); err != nil {
			log.Warnf("set cgroup controller %v  fail: %v", controller.Name(), err)
		}
	}
}

func (c *CgroupV2Manager) Destroy() {
	os.RemoveAll(c.dirPath)
	os.Remove(c.dirPath)
}
```

每个方法的实现就按照接口的定义来实现即可，唯一值得唠叨的是 `Set()` 方法，它要遍历 Controllers 列表，调用每个 Controller 并调用 `Set()` 方法来设置 cgroup 的资源限制。

之后，我们还需要提供一个方法来创建 CgroupV2Manager（当然，在需要用到 CgroupV2Manager 的地方现场定义一个 CgroupV2Manager 结构体也不是不行）

```go
const unifiedMountPoint = "/sys/fs/cgroup"

func NewCgroupV2Manager(dirPath string) *CgroupV2Manager {
	if !strings.HasPrefix(dirPath, unifiedMountPoint) {
		dirPath = path.Join(unifiedMountPoint, dirPath)
	}

	return &CgroupV2Manager{
		dirPath:     dirPath,
		controllers: Controllers,
	}
}
```

这里我们将 cgroup 目录统一放在 `/sys/fs/cgroup` 下，这样就可以保证 cgroup 的目录是统一的（想要修改这个路径也是可以的，但是要确保在 `/sys/fs/cgroup` 的子目录中，不然起不到作用）

至此，对 Cgroup 的封装基本就完成了，剩下只需要在 run 命令中调用即可。

### cmd/run.go

```go
func run(tty bool, comArray []string, resConf *resource.ResourceConfig) {
	process, writePipe := libcontainer.NewContainerProcess(tty)
	if process == nil {
		log.Errorf("New process error!")
		return
	}

	if err := process.Start(); err != nil {
		log.Errorf("Run process.Start() err: %v", err)
	}

	// 创建 cgroup manager
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

	sendInitCommand(comArray, writePipe)
	_ = process.Wait()
}
```

核心代码如下：

```go
// 创建 cgroup manager
cgroupManager, err := cgroup.NewCgroupManager("m-docker.slice")
// 当前函数 return 后释放 cgroup
defer cgroupManager.Destroy()
if err != nil{
	...
}
// 初始化 cgroup
if err = cgroupManager.Init(); ... {
	...
}
// 将子进程加入到 cgroup 中
if err = cgroupManager.Apply(process.Process.Pid); ... {
	...
}
// 设置 cgroup 的资源限制
cgroupManager.Set(resConf)
```

创建一个 CgroupManager，并调用 `Init()` 方法进行初始化 cgroup，再调用 `Apply()` 方法将容器进程加入到 cgroup 中，最后调用 `Set()` 方法设置 cgroup 的资源限制。同时，`defer cgroupManager.Destroy()` 保证了在容器进程结束后（即当前函数返回时）释放 cgroup。这样，我们就完成了 cgroup 的全部设置。

`cgroup.NewCgroupManager` 方法会根据 cgroup 版本创建不同的 cgroup manager，目前我们只实现了 cgroup v2 的 manager，如果后续考虑兼容 cgroup v1 的话，可以在实现了 CgroupV1Manager 之后，在这里进行修改。

### libcontainer/cgroup/cgroup.go

```go
// 根据 cgroup 版本创建 CgroupManager
func NewCgroupManager(dirPath string) (Cgroup, error) {
	// 如果支持 cgroup v2，则使用 cgroup v2
	if IsCgroup2UnifiedMode() {
		log.Infof("using cgroup v2")
		return v2.NewCgroupV2Manager(dirPath), nil
	}
	// 目前不考虑支持 cgroup v1，因此直接返回错误
	return nil, fmt.Errorf("cgroup v2 is not supported")
}
```

这里我们调用了 `IsCgroup2UnifiedMode()` 方法来判断当前系统是否支持 cgroup v2，它的实现如下：

### libcontainer/cgroup/utils.go

```go
const (
	unifiedMountPoint = "/sys/fs/cgroup"
)

var (
	isUnifiedOnce sync.Once // sync.Once 用于确保某种操作只进行一次
	isUnified     bool
)

// IsCgroup2UnifiedMode 检查 cgroup v2 是否启用
func IsCgroup2UnifiedMode() bool {
	// 使用 sync.Once 来确保检查 cgroup v2 的操作只进行一次
	// 目的只是为了提高性能，避免重复检查
	isUnifiedOnce.Do(func() {
		var st unix.Statfs_t
		err := unix.Statfs(unifiedMountPoint, &st)

		// 如果 unifiedMountPoint 不存在，则 cgroup v2 肯定未启用
		if err != nil && os.IsNotExist(err) {
			isUnified = false
		} else { // 若 unifiedMountPoint 存在，则还需要根据目录类型判断 cgroup v2 是否启用
			isUnified = (st.Type == unix.CGROUP2_SUPER_MAGIC)
		}
	})
	return isUnified
}
```

只需要判断 `/sys/fs/cgroup` 目录的类型是否为 cgroup2 即可。我们使用了 `sync.Once` 来确保检查只会进行一次，这样可以避免重复检查，提高性能。

### 测试

我们测试一下 cpu 占用：
- 启动容器并使用 `-cpu` 限制 cpu 使用率
- 运行一个死循环进程，看看进程的 cpu 使用率是否超过限制

```bash
# sudo ./m-docker run -it -cpu 0.5 /bin/bash
{"level":"info","msg":"using cgroup v2","time":"2024-07-20T09:11:57Z"}
{"level":"info","msg":"Set cgroup cpu.max: 50000 100000","time":"2024-07-20T09:11:57Z"}
{"level":"info","msg":"Set cgroup memory.max: max","time":"2024-07-20T09:11:57Z"}
{"level":"info","msg":"Send command to init: /bin/bash","time":"2024-07-20T09:11:57Z"}
{"level":"info","msg":"--- Inside the container ---","time":"2024-07-20T09:11:57Z"}
{"level":"info","msg":"Start func: initContainer","time":"2024-07-20T09:11:57Z"}
{"level":"info","msg":"find command path: /bin/bash","time":"2024-07-20T09:11:57Z"}
```

在后台运行一个死循环进程：

```bash
# while :; do :; done &
[1] 11
```

理论上，该 while 循环会占用满一整个 cpu，但是该容器被限制了只能占用 0.5 核 cpu，因此最终该 while 循环会被限制在 0.5 核。

查看进程的 cpu 使用率：

```bash
# top -p 11
    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND                                                                                        
     11 root      20   0    7768    704      0 R  50.3   0.0   0:15.04 bash
```

成功！cpu 使用率果然被限制在 50% 左右了。

内存的

## 小结

整个 cgroup 的实现并不算复杂，只要理清了原理，写起来很简单：
- **找到 cgroup 的文件系统挂载路径**：cgroup v2 的挂载路径在 `/sys/fs/cgroup`，创建一个子目录便是创建新的 cgroup。
- **确定接口文件**：
  - 进程列表：`cgroup.procs` 文件
  - cpu 使用率：`cpu.max` 文件
  - 内存占用：`memory.max` 文件
- **往对应接口文件写入限制信息**：很简单的文件写入操作。