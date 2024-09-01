# chapter10 - ps 命令实现

前面我们已经实现了 `run` 命令，但是光能够运行容器还不够，我们还需要能够查看已有的容器。所以这一节，我们将实现 `ps` 命令。

## 1. 实现思路

`ps` 命令的功能是列出当前正在运行的容器，包括容器的 ID、名称、创建时间、状态等信息。若添加了参数 `--all, -a`，则可以列出所有容器，包括已经退出的容器（但不包括已经清除的容器）。

我们知道容器在运行之后会在 `/run/m-docker/<容器ID>` 这个路径下创建状态目录，我们可以遍历 `/run/m-docker` 目录下的所有目录，这些就是正在运行/已经退出但还没清除的容器。然后我们可以读取这些目录下的配置文件 `config.json`，获取容器的信息。

## 2. 具体实现

首先我们需要定义 `ps` 子命令：

### cmd/ps.go

```go
// m-docker ps 命令
var ContainerListCommand = cli.Command{
	Name:      "ps",
	Usage:     `show the containers in list`,
	UsageText: `m-docker ps`,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "all, a", // 显示所有容器
			Usage: "show all containers",
		},
	},

	Action: func(context *cli.Context) error {
		if err := listContainers(context); err != nil {
			return fmt.Errorf("list containers error: %v", err)
		}
		return nil
	},
}
```

可以看到 `ps` 命令的入口函数非常简单，仅仅是调用 `listContainers` 函数。

这个函数需要完成的事情也相对明确：遍历 `/run/m-docker` 目录下的所有目录，读取这些目录下的 `config.json` 文件，并在命令行上输出容器的信息。

```go
// 查询 m-docker 状态目录下的所有目录，根据 config.json 文件获取容器信息
func listContainers(ctx *cli.Context) error {
	// 读取状态目录下的所有容器目录
	files, err := os.ReadDir(constant.StatePath)
	if err != nil {
		return fmt.Errorf("read dir %s error: %v", constant.StatePath, err)
	}

	// 遍历所有容器目录，获取容器 Config
	containersConfigs := make([]*config.Config, 0, len(files))
	for _, file := range files {
        // 调用 GetConfigFromID 函数获取容器的 Config
		conf, err := config.GetConfigFromID(file.Name())
		if err != nil {
			log.Warningf("get config from id %s error: %v", file.Name(), err)
			continue
		}
		if ctx.Bool("all") || conf.Status == constant.ContainerRunning {
			containersConfigs = append(containersConfigs, conf)
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	_, err = fmt.Fprintf(w, "CONTAINER ID\tPID\tCOMMAND\tCREATED\tSTATUS\tNAME\n")
	if err != nil {
		return fmt.Errorf("failed to execute fmt.Fprintf: %v", err)
	}
	for _, item := range containersConfigs {
		_, err = fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\n",
			item.ID[:12],  // 为了输出不显得臃肿，只显示前 12 位
			item.Pid,
			strings.Join(item.CmdArray, " "),
			item.CreatedTime,
			item.Status,
			item.Name,
		)
		if err != nil {
			return fmt.Errorf("failed to execute fmt.Fprintf: %v", err)
		}
	}
	w.Flush()

	return nil
}
```

`listContainers` 函数调用了 `config.GetConfigFromID` 方法来获取每个容器的 `Config`；之后判断容器的运行状态是否为 `Running`，来决定是否进行展示；最后通过 `tabwriter` 来格式化输出容器的信息，这里为了输出不显得臃肿，容器 ID 只显示前 12 位（不用担心不同的容器 ID 前 12 位会相同，这个情况确实有，但是期望概率是 **$\frac{1}{16^{12}}$**）

这里我们有 2 个前置需求还没有实现：

- 添加容器的运行状态
- 实现 `config.GetConfigFromID` 方法：

这里我们先给容器添加运行状态：

### libcontainer/constant/status.go

```go
const (
	ContainerRunning = "Running"
	ContainerStopped = "Stopped"
)
```

我们创建了一个 `status.go` 文件，定义了容器的运行状态，目前只有 `Running` 和 `Stopped` 两种状态。

### libcontainer/container.go

```go
// 启动容器
func (c *Container) Start() error {
	// 省略...

	// 启动容器进程
	if err := process.Start(); err != nil {
		return fmt.Errorf("failed to run process.Start(): %v", err)
	}
	c.Config.Pid = process.Process.Pid
	c.Config.Status = constant.ContainerRunning

	// 省略...

	return nil
}
```

我们在容器启动后为容器添加状态 `Running` 即可。

之后我们实现 `config.GetConfigFromID` 方法：

### libcontainer/config/utils.go

```go

// 根据容器状态目录路径获取容器 Config
func GetConfigFromStatePath(statePath string) (*Config, error) {
	configPath := path.Join(statePath, constant.ConfigName)
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", configPath, err)
	}

	conf := new(Config)
	if json.Unmarshal(content, conf) != nil {
		return nil, fmt.Errorf("failed to unmarshal json content: %v", err)
	}
	return conf, nil
}

// 根据容器 ID 获取容器 Config
func GetConfigFromID(id string) (*Config, error) {
	if len(id) == 12 {
		files, err := os.ReadDir(constant.StatePath)
		if err != nil {
			return nil, fmt.Errorf("read dir %s error: %v", constant.StatePath, err)
		}

		for _, file := range files {
			if strings.HasPrefix(file.Name(), id) {
				id = file.Name()
				break
			}
		}
	} else if len(id) != 64 {
		return nil, fmt.Errorf("invalid container ID")
	}

	statePath := path.Join(constant.StatePath, id)
	return GetConfigFromStatePath(statePath)
}
```

`GetConfigFromID` 方法要先判断传入的容器 ID 是否合法（12位或64位）。若 ID 为 12 为，则遍历 `/run/m-docker` 目录下的所有目录，找到对应的完整容器 ID。之后调用 `GetConfigFromStatePath` 方法获取容器的 `Config`。

`GetConfigFromStatePath` 方法则更简单，直接读取容器状态目录下的 `config.json` 文件，然后通过 `json.Unmarshal` 方法解析为 `Config` 对象即可。

## 3. 测试

我们创建一个容器，让其运行 `sleep 60` 命令阻塞 60 秒：

```bash
# go build && sudo ./m-docker run -d 'sleep 60'

```

之后我们运行 `ps` 命令：

```bash
# sudo ./m-docker ps
CONTAINER ID   PID         COMMAND     CREATED               STATUS      NAME
ba32854a38fc   2568577     sleep 60    2024-09-01 22:44:27   Running     confident_banach
```

可以看到我们的容器信息已经成功展示出来了。

我们等待 60 秒，此时容器运行结束，shim 进程会清理容器的所有数据目录，我们再次运行 `ps` 命令：

```bash
# sudo ./m-docker ps
CONTAINER ID   PID         COMMAND     CREATED               STATUS      NAME
```

可以看到容器已经不在列表中，符合我们的预期。