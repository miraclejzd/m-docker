# chapter 11 - logs 命令实现

这一节我们继续完善容器运行时的功能，让它支持查看容器的日志。

## 1. 实现思路

我们的容器是通过 `exec.Command` 来启动的，其中 `cmd.Stdout` 和 `cmd.Stderr` 分别是标准输出和标准错误输出，它们只需要是 `io.Writer` 接口的实现即可。因此，我们可以将这两个输出重定向到某个文件中，这样进程的所有标准输出和标准错误输出都会被写入到这个文件中，这个文件就是我们的日志文件。

想获取某个容器的日志，只需要读取这个容器的日志文件即可。

我们还需要再明确一个问题：所有容器都会有日志文件吗？

按照 Docker 的设计，是的。但是在我们这里，**不是**。

因为我们实现的方式是将 `cmd.Stdout` 和 `cmd.Stderr` 重定向到文件中，那么如果容器正在前台运行，当前终端的输出被重定向到文件中，我们就没办法在前台看到容器的反馈了。因此，在这种实现方式下，我们只能对后台运行的容器进行日志的重定向。

> 如何实现 Docker 那样的全容器日志输出呢？不妨当做一个思考题吧哈哈，有实现思路的话欢迎PR～

## 2. 具体实现

我们首先要为容器 `Config` 添加一个属性，用来存储日志文件的路径：

### libcontainer/config/config.go

```go
// 包含了容器的所有配置信息
type Config struct {
	// 省略...

	// 容器的日志文件路径
	LogPath string `json:"logPath"`

	// 省略...
}
```

同时我们要在 `constant` 包里设置容器日志的名称：

### libcontainer/constant/path.go

```go
const (
	// 省略

	// 容器日志文件名
	LogFileName = "log.json"
)
```

之后，在创建容器 `Config` 的时候，添加日志文件路径：

### libcontainer/config/utils.go

```go
// 生成容器的 Config 配置
func CreateConfig(ctx *cli.Context) (*Config, error) {
	// 省略...  

	return &Config{
		// 省略...
		LogPath:     path.Join(constant.StatePath, containerID, constant.LogFileName),
		// 省略...
	}, nil
}
```

之后我们需要在容器进程启动的时候，将容器的标准输出和标准错误输出重定向到日志文件中：

### libcontainer/container.go

```go
// 生成一个容器进程的句柄
// 该容器进程将运行 m-docker init ，并拥有新的 UTS、PID、Mount、NET、IPC namespace
func newContainerProcess(conf *config.Config) (*exec.Cmd, *os.File, error) {
	// 省略...

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

	// 省略...
}
```

这里值得注意的有两点：

1. 当前这个函数调用的时候，容器的状态目录 `stateDir` 还没有创建，这时我们要先创建状态目录再创建日志文件。
2. 创建日志文件后，不要关闭文件描述符。此时容器进程还没有启动，关闭了文件描述符的话，容器进程就无法访问这个文件了。

完成后我们就可以设计 `logs` 子命令了：

### cmd/logs.go

```go
// m-docker logs 命令
var LogsCommand = cli.Command{
	Name:      "logs",
	Usage:     `fetch the logs of a container`,
	UsageText: `m-docker logs CONTAINER`,

	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("\"m-docker run\" requires at least 1 argument")
		}

		// 获取容器 ID
		c := context.Args().Get(0)
		id, err := config.GetIDFromName(c)
		if err != nil {
			id, err = config.GetIDFromPrefix(c)
			if err != nil {
				return fmt.Errorf("container %s not found", c)
			}
		}

		// 打印容器日志
		err = logContainer(id)
		if err != nil {
			return fmt.Errorf("failed to log container %s: %v", id, err)
		}

		return nil
	},
}
```

`logs` 命令可以接受2种参数：
1. 容器的名称
2. 容器 ID 的前缀。

这里我们可以先尝试通过容器的名称获取完整的 sha-256 ID，如果失败再通过容器的前缀获取，若查询到了则调用 `logContainer` 函数打印容器的日志。

```go 
// 查询容器的日志文件，并打印
func logContainer(id string) error {
	// 获取容器 Config
	conf, err := config.GetConfigFromID(id)
	if err != nil {
		return fmt.Errorf("failed to get container config: %v", err)
	}

	// 读取容器日志文件
	logContent, err := os.ReadFile(conf.LogPath)
	if err != nil {
		return fmt.Errorf("failed to read log file: %v", err)
	}

	// 打印日志内容至标准输出
	_, err = fmt.Fprint(os.Stdout, string(logContent))
	if err != nil {
		return fmt.Errorf("failed to print log content: %v", err)
	}

	return nil
}
```

`logContainer` 函数通过容器的完整 ID 获取容器 `Config`，然后将容器的日志文件打印至标准输出。

## 3. 测试

我们准备一个脚本，隔 10s 打印一次 `hello world`：

### scrpits/cronjob.sh

```bash
#!/bin/bash

while true; do
    echo "$(date '+%Y-%m-%d %H:%M:%S') hello world"
    sleep 10
done
```

之后我们创建一个容器后台运行这个脚本：

```bash
# go build && sudo ./m-docker run -d --name cronjob -v /home/jzd/projects/m-docker/scripts:/data "bash /data/cronjob.sh"
ba69267c77676f1290966eff8cbf2328bed430ac581586e39cb12ce20abac793
```

然后我们通过 `ps` 命令查看容器的状态：

```bash
# ./m-docker ps
CONTAINER ID   PID         COMMAND                 CREATED               STATUS      NAME
ba69267c7767   3856881     bash /data/cronjob.sh   2024-09-13 17:39:30   Running     cronjob
```

嗯哼～容器正常地运行着。

我们可以通过 `logs` 命令查看容器的日志：

```bash
# ./m-docker logs cronjob
2024-09-13 09:39:30 hello world

# /m-docker logs ba69267c7767
2024-09-13 09:39:30 hello world
2024-09-13 09:39:40 hello world
```

通过名字和 ID 前缀都可以查看到容器的日志！

最后别忘了将容器删除：

```bash 
# sudo kill -9 3856881
(空)
```