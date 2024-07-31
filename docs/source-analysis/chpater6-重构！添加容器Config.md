# chapter6 - 重构！添加容器 Config

前面 5 节中，我们已经实现了一个简单的容器，麻雀虽小，五脏俱全，如果仅仅想探究容器的实现原理，那么这个麻雀容器就已经足够了。

从这一节开始，我们将开始对容器进行一些进阶的操作。

细心的读者可能发现了，前面我们实现的麻雀容器全局只有一个，因此许多配置例如 rootfs 路径、Cgroup 路径都是硬编码写死的，当容器运行结束后，这些数据目录都会被直接删除，下次运行新容器时再复用相同的路径。

如果我们后续考虑实现容器的后台运行、容器的网络等功能，至少需要支持多个容器同时运行，那么当前这种只支持单个容器的实现方式就不够用了。

于是，我们重构的第一步，便是为容器添加配置信息，将不同的容器区分开来。

## 1. 实现思路

我们首先需要定义一个 Config 结构体，用于存储容器的全部配置信息。每次收到容器创建的请求时，我们生成配置信息，并根据配置信息创建容器。

至于配置信息的持久化存储，我们在 `/run/m-docker` 目录下创建一个以容器 ID 为名的子目录作为该容器的状态信息目录，并将该容器的配置信息存储在这个状态信息目录下。

## 2. 具体实现

我们首先定义一个 Config 结构体，用于存储容器的全部配置信息：

### libcontainer/config/config.go

```go
// 包含了容器的所有配置信息
type Config struct {
	// 容器的运行状态
	Status string `json:"status"`

	// 容器的进程在宿主机上的 PID
	Pid int `json:"pid"`

	// 容器的唯一标识符
	ID string `json:"ID"`

	// 容器名称
	Name string `json:"name"`

	// 容器的 rootfs 路径
	Rootfs string `json:"rootfs"`

	// 容器的读写层路径
	RwLayer string `json:"rwLayer"`

	// 容器的状态信息路径
	StateDir string `json:"stateDir"`

	// 容器是否启用 tty
	TTY bool `json:"tty"`

	// 容器的运行命令
	CmdArray []string `json:"CmdArray"`

	// 容器的 Cgroup 配置
	Cgroup *Cgroup `json:"cgroup"`

	// 容器的创建时间
	CreatedTime string `json:"createdTime"`
}
```

其中的 Cgroup 对象定义如下：

```go
type Cgroup struct {
	// cgroup 名称
	Name string `json:"name"`

	// cgroup 目录的绝对路径
	Path string `json:"path"`

	*Resources
}

// cgroup 资源限制
type Resources struct {
	// 内存限制
	Memory string

	// CPU 硬限制(hardcapping)的调度周期
	CpuPeriod uint64 `json:"cpuPeriod"`

	// 在 CPU 硬限制的调度周期内，期望使用的 CPU 时间
	CpuQuota uint64 `json:"cpuQuota"`
}
```

接着我们需要提供一个供外部调用的函数 `CreateConfig` 用来创建 Config 对象：

### libcontainer/config/utils.go

```go
const (
	// m-docker 数据的根目录
	rootPath = "/var/lib/m-docker"

	// m-docker 状态信息的根目录
	statePath = "/run/m-docker"
)

// 生成容器的 Config 配置
func CreateConfig(ctx *cli.Context) *Config {
	// 容器创建时间
	createdTime := time.Now().Format("2024-07-30 00:28:58")

	// 从命令行参数中获取容器名称
	containerName := ctx.String("name")
	// 如果没有设置容器名称，则生成一个随机名称
	if containerName == "" {
		containerName = generateContainerName()
	}

	// 生成容器ID
	containerID := generateContainerID(containerName + createdTime)

	// 获取容器的运行命令
	var cmdArray []string
	if ctx.NArg() < 1 {
		log.Warnf("missing container command, filling with '/bin/bash' ")
		cmdArray = append(cmdArray, string("/bin/bash"))
	} else {
		for _, arg := range ctx.Args() {
			cmdArray = append(cmdArray, arg)
		}
	}

	return &Config{
		ID:          containerID,
		Name:        containerName,
		Rootfs:      path.Join(rootPath, "rootfs", containerID),
		RwLayer:     path.Join(rootPath, "layers", containerID),
		StateDir:    path.Join(statePath, containerID),
		TTY:         ctx.Bool("it"),
		CmdArray:    cmdArray,
		Cgroup:      createCgroupConfig(ctx, containerID),
		CreatedTime: createdTime,
	}
}
```
其中：
- `Rootfs`：`/var/lib/m-docker/rootfs/<containerID>`
- `StateDir`：`/run/m-docker/<containerID>`

`generateContainerName` 和 `generateContainerID` 函数用于生成容器名称和容器 ID：

```go
// 生成容器ID
func generateContainerID(input string) string {
	hash := sha256.New()
	hash.Write([]byte(input))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// 预定义的形容词列表
var adjectives = []string{
	"admiring", "adoring", "affectionate", "agitated", "amazing",
	"angry", "awesome", "blissful", "boring", "brave",
	"charming", "clever", "cool", "compassionate", "competent",
	"confident", "cranky", "crazy", "dazzling", "determined",
}

// 预定义的名词列表
var nouns = []string{
	"albattani", "allen", "almeida", "agnesi", "archimedes",
	"ardinghelli", "aryabhata", "austin", "babbage", "banach",
	"banzai", "bardeen", "bartik", "bassi", "beaver",
	"bell", "benz", "bhabha", "bhaskara", "blackwell",
}

// 生成随机容器名称
func generateContainerName() string {
	rand.Seed(uint64(time.Now().UnixNano()))
	adj := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	return fmt.Sprintf("%s_%s", adj, noun)
}
```

值得注意的是，目前容器 ID 是根据容器名与容器创建时间所生成的 SHA256 哈希值，能否保证 ID 完全唯一还有待验证，这里我们暂且先这么实现。

`createCgroupConfig` 函数则是用于生成 Cgroup 配置：

```go
const (
	// 默认 CPU 硬限制调度周期为 100000us
	defaultCPUPeriod = 100000

	// cgroup 根目录
	cgroupRootPath = "/sys/fs/cgroup/m-docker.slice"

)

// 生成 cgroup 配置
func createCgroupConfig(ctx *cli.Context, containerID string) *Cgroup {
	name := "m-docker-" + containerID

	return &Cgroup{
		Name:      name,
		Path:      path.Join(cgroupRootPath, name+".scope"),
		Resources: createCgroupResource(ctx),
	}
}

// 生成 cgroup 资源配置
func createCgroupResource(ctx *cli.Context) *Resources {
	// 内存限制
	memory := ctx.String("mem")
	if memory == "" {
		memory = "max"
	}

	// cpu 使用率限制
	var cpuQuota uint64
	cpuPercent := ctx.Float64("cpu")
	if cpuPercent == 0 {
		cpuQuota = 0
	} else {
		cpuQuota = uint64(cpuPercent * defaultCPUPeriod)
	}

	return &Resources{
		Memory:    memory,
		CpuPeriod: defaultCPUPeriod,
		CpuQuota:  cpuQuota,
	}
}
```

其中：
- `cgroupPath`: `/sys/fs/cgroup/m-docker.slice/m-docker-<containerID>.scope`

配置信息生成完毕后，我们还需要提供函数 `RecordContainerConfig` 供外部调用，将配置信息持久化存储到宿主机的磁盘：

```go
// 容器 Config 文件名
const	configName = "config.json"

// 将容器的 Config 持久化存储到磁盘上
func RecordContainerConfig(conf *Config) error {
	// 创建容器的状态信息目录
	if err := os.MkdirAll(conf.StateDir, 0777); err != nil {
		return fmt.Errorf("failed to create container state dir:  %v", err)
	}

	// 将容器 Config 写入文件
	jsonBytes, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("failed to marshal container config:  %v", err)
	}
	jsonStr := string(jsonBytes)
	filePath := path.Join(conf.StateDir, configName)
	file, err := os.Create(filePath)
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorf("failed to close file %v:  %v", filePath, err)
		}
	}()
	if err != nil {
		return fmt.Errorf("failed to create file %s:  %v", filePath, err)
	}
	if _, err = file.WriteString(jsonStr); err != nil {
		return fmt.Errorf("failed to write container config to file %s:  %v", filePath, err)
	}

	return nil
}
```

其中：
- `filePath`: `/run/m-docker/<containerID>/config.json`

同时我们还需要提供 `DeleteContainerConfig` 函数，用于删除容器的配置信息：

```go
// 删除容器的状态信息
func DeleteContainerState(conf *Config) {
	os.RemoveAll(conf.StateDir)
}
```

至此，我们已经把准备工作完成了，接下来只需要重构已有的代码，让它们使用 Config 对象作为参数，并容器创建/退出的时候生成/删除 Config 即可。

### cmd/run.go

```go
var RunCommand = cli.Command{
	// 省略...

	Action: func(context *cli.Context) error {
		// 生成容器的配置信息
		conf := config.CreateConfig(context)

		run(conf)

		return nil
	},
}

func run(conf *config.Config) {
	if err := libcontainer.CreateRootfs(conf); err != nil {
		// 省略...
	}

	process, writePipe := libcontainer.NewContainerProcess(conf)

	// 省略...

	conf.Pid = process.Process.Pid
	// 将容器的配置信息持久化到磁盘上
	if err := config.RecordContainerConfig(conf); err != nil {
		log.Errorf("Record container config error: %v", err)
		return
	}

	cgroupManager, err := cgroup.NewCgroupManager(conf.Cgroup.Path)
	defer func() {
		libcontainer.DeleteRootfs(conf)

		cgroupManager.Destroy()

		// 删除容器的状态信息
		config.DeleteContainerState(conf)
	}()
	
	// 省略...

	cgroupManager.Set(conf.Cgroup.Resources)

	// 省略...
}
```

细心的观众可能注意到了，在容器退出之后，我们目前会将所有容器的数据、状态信息都删除。实际上 Docker 并不是这么做的，只要我们没有指定 `--rm` 参数，那么容器退出之后的数据是不会被删除的。这个功能我们会在后续的章节中实现，目前暂且先这么处理。

其余文件的变动也是类似的，这里就不一一列举了。

还有个与这一章节无关的改动，就是我将所有 `Info` 级别的日志都改为 `Debug` 级别，这样在运行容器时就不会看到太多无关的日志了。同时我为 `m-docker` 命令添加了全局的 `--debug` 参数，在命令里加上这个参数就可以看到所有 `Debug` 级别的日志了。

### main.go

```go
func main() {
	//省略...

	// 全局 flag
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug", // 启用 debug 模式
			Usage: "enable debug mode",
		},
	}
	app.Before = func(context *cli.Context) error {
		log.SetFormatter(&log.JSONFormatter{})

		// 设置日志级别
		if context.Bool("debug") {
			log.SetLevel(log.DebugLevel)
		}

		log.SetOutput(os.Stdout)
		return nil
	}
	// 省略...

}
```

## 3. 测试

我们重新创建一个容器，并且启用 `--debug` 参数：

```bash
# sudo ./m-docker --debug run -it
{"level":"warning","msg":"missing container command, filling with '/bin/bash' ","time":"2024-07-31T13:58:48Z"}
{"level":"debug","msg":"Mount overlay command: /usr/bin/mount -t overlay m-docker-overlay -o lowerdir=/var/lib/m-docker/layers/ubuntu,upperdir=/var/lib/m-docker/layers/b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28/fs,workdir=/var/lib/m-docker/layers/b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28/work /var/lib/m-docker/rootfs/b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28","time":"2024-07-31T13:58:48Z"}
{"level":"debug","msg":"using cgroup v2","time":"2024-07-31T13:58:48Z"}
{"level":"debug","msg":"Set cgroup cpu.max: max 100000","time":"2024-07-31T13:58:48Z"}
{"level":"debug","msg":"Set cgroup memory.max: max","time":"2024-07-31T13:58:48Z"}
{"level":"debug","msg":"Send command to init: /bin/bash","time":"2024-07-31T13:58:48Z"}
root@master-58:/# 
```

我们退出容器，再次创建一个容器，这次不启用 `--debug` 参数：

```bash
# go build && sudo ./m-docker run -it
{"level":"warning","msg":"missing container command, filling with '/bin/bash' ","time":"2024-07-31T13:57:55Z"}
root@master-58:/# 
```

果然，日志干净了很多。

接着我们重新打开一个终端，查看宿主机上容器的状态信息是否保存：

```bash
# sudo ls /run/m-docker
b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28
# sudo cat /runc/m-docker/b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28/config.json
{"status":"","pid":3106234,"ID":"b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28","name":"compassionate_bartik","rootfs":"/var/lib/m-docker/rootfs/b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28","rwLayer":"/var/lib/m-docker/layers/b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28","stateDir":"/run/m-docker/b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28","tty":true,"CmdArray":["/bin/bash"],"cgroup":{"name":"m-docker-b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28","path":"/sys/fs/cgroup/m-docker.slice/m-docker-b8b84e87f4121b5a7d9748e4913ff2b1cb928e4d7198cda15774b2ae50f66f28.scope","Memory":"max","cpuPeriod":100000,"cpuQuota":0},"createdTime":"313158+00-10 00:318:488"}
```

没错了，我们的容器状态信息已经保存到了磁盘上。

接着我们退出容器，再次查看状态信息：

```bash
# sudo ls /run/m-docker
（空）
```

果然，容器退出之后，状态信息也被删除了。