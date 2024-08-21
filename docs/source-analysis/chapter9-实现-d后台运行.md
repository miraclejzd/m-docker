# chapter9 - 实现 -d 后台运行

这一节我们将为我们的容器添加 `-d` 参数，使得容器进程可以在后台运行。

## 1. 实现思路

我们所希望的容器后台运行本质上是：键入 `m-docker run` 命令后，当前终端可以立即返回，而容器进程在后台运行。

那么我们就需要看看目前前台运行的实现里，终端会如何被占用。

我们在终端键入 `m-docker run` 命令后会创建 `m-docker run` 进程，它负责容器生命周期的管理。它将会调用 `container.Start()` 创建容器进程，并会一直阻塞，等待容器进程退出。这个过程中，终端的“所有权”会被 `m-docker run` 进程所占用，如果它退出了，那么终端就会被释放。

所以，我们只需要将前台的 `m-docker run` 进程退出，终端就会被释放。

但是有这么一个问题：`m-docker run` 进程是管理容器生命周期的进程，理论上需要与容器的生命周期一致。举个例子，`m-docker run` 进程会在容器进程退出后调用 `container.Remove()` 方法来清理容器资源。如果我们让 `m-docker run` 进程退出了，那么 `container.Remove()` 方法就不会被调用，容器资源就不会被清理。因此 `m-docker run` 进程至少需要等待容器进程退出后再退出。

这似乎是个死结，要想让容器进程在后台运行，就必须让 `m-docker run` 进程退出从而释放终端，但是 `m-docker run` 进程退出了，容器的生命周期就不能被正确的管理。

有什么解决办法吗？

当然有。我们可以通过 `fork` 系统调用将 `m-docker run` 进程复制一份，让这个复制的进程去管理容器的生命周期，而让原来占据着终端的 `m-docker run` 进程退出。这样，终端就可以被释放，而容器的生命周期又可以被正确的管理。

还有一个更深入的细节，不过点到为止即可：当原来的 `m-docker run` 进程退出后，`fork` 创建的进程由于失去了父进程，会被 `init` 进程（PID 为 1 的进程）接管，因此不会成为孤儿进程。

> 这个方法就是 Docker 早期的实现，早期 Docker 容器的生命周期管理进程都是从 Docker daemon 进程 fork 出来的。

## 2. 具体实现

我们只需要在 `m-docker run` 命令的实现里增添 `fork` 的逻辑即可。

### cmd/run.go

```go
// m-docker run 命令
var RunCommand = cli.Command{
    // 省略...
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "d, detach", // 后台运行
			Usage: "detach container",
		},
        // 省略...
	},

	Action: func(context *cli.Context) error {
		// 省略...

		// 若为前台运行，则由当前这个 m-docker run 进程直接管理容器生命周期
		// 启动容器进程后，当前进程会阻塞，等待容器运行结束
		if conf.TTY {
			run(conf)
		} else { // 后台运行
			// 打印容器 ID
			fmt.Printf("%v\n", conf.ID)

			// fork 一个进程作为 shim 来管理容器生命周期
			// 之后当前这个 m-docker run 进程就可以退出了
			pid, _, errno := syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
			if errno != 0 {
				return fmt.Errorf("fork error: %v", err)
			}

			// 子进程
			if pid == 0 {
				log.Debugf("[shim process] fork success")
				run(conf)
			} else { // 父进程
				log.Debugf("[father process]fork shim process, pid: %d", pid)
			}
		}

		return nil
	},
}
```

只需要通过 `syscall` 方法来调用 `fork` 系统调用即可。值得注意的是 `fork` 后的子进程和父进程的程序计数器是一样的，因此子进程也会执行 `syscall` 后的语句，所以我们需要判断一下当前是子进程还是父进程，让它们执行不同的逻辑。

随后我们还需要修改 `config.CreateConfig()` 方法，让其能处理 `-d` 参数。

### libcontainer/config/utils.go

```go
// 生成容器的 Config 配置
func CreateConfig(ctx *cli.Context) (*Config, error) {
	// 省略...

	// 判断容器在前台运行还是后台运行
	tty := ctx.Bool("it")
	detach := ctx.Bool("detach")
	if tty && detach { // 特判同时设置 -it 和 -d 的情况
		return nil, fmt.Errorf("it and detach can not be set at the same time")
	}
	// 这里并不需要判断 detach 为 true 的情况，因为 detach 为 true 时，tty 必为 false

	// 省略...
}
```

由于 `-it` 和 `-d` 是互斥的，这里我们只需要添加一个特判即可。

## 3. 测试

我们运行一个持续运行的容器：

```bash
# go build && sudo ./m-docker --debug run -d "sleep infinity"
8165fe6c6f59fe29a0247ac0bf2337abb21cb72308bd3f1bf88b58e9ef4620ba
DEBU[2024-08-21T17:58:33Z] [father process] fork shim process, pid: 1395514 
DEBU[2024-08-21T17:58:33Z] [shim process] fork success 
```

可以看到，我们 `fork` 出来的 shim 进程的 pid 为 1395514。

我们 `pstree` 查看一下：

```bash
# sudo pstree -sp 1395514
systemd(1)───m-docker(1395514)───sleep(1395517)
```

果然，shim 进程创建了容器进程（PID 为 1395517），同时由于原来 `m-docker run` 的退出，shim 进程的父进程变成了 `init` 进程（PID 为 1）。

随后我们查看 `/run/m-docker` 目录，看看有没有容器的状态信息：

```bash
# ls /run/m-docker
8165fe6c6f59fe29a0247ac0bf2337abb21cb72308bd3f1bf88b58e9ef4620ba
```

果然有，我们查看一下 `config.json`：

```bash
# cat /run/m-docker/8165fe6c6f59fe29a0247ac0bf2337abb21cb72308bd3f1bf88b58e9ef4620ba/config.json
{
  "status": "",
  "pid": 1395517,
  "ID": "8165fe6c6f59fe29a0247ac0bf2337abb21cb72308bd3f1bf88b58e9ef4620ba",
  "name": "adoring_aryabhata",
  "rootfs": "/var/lib/m-docker/rootfs/8165fe6c6f59fe29a0247ac0bf2337abb21cb72308bd3f1bf88b58e9ef4620ba"
  ...
}
```

可以看到，容器进程的 PID 为 1395517，与上面 `pstree` 的结果一致。

我们删除容器进程，看看 shim 进程是否会清理容器资源：

```bash
# sudo kill -9 1395517
（空）

# ls /run/m-docker
（空）
```

太对了，shim 进程清理了容器资源，容器的生命周期被正确的管理了。