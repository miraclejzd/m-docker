# chapter7 - 实现 -v 数据卷挂载

之前我们基于 overlay 为容器实现了独立的 rootfs。

**如果用户想将数据持久化，或是想复用宿主机的数据，该怎么办？**

在 Docker 里，我们可以通过 `-v` 参数来创建 volume（数据卷）来实现数据的持久化。

这一节，我们就要实现将宿主机目录作为 volume 挂载到容器中，这样在容器退出后，volume 中的内容仍然能持久化地保存在宿主机上。

## 1. 实现思路

这个操作的具体实现依赖于 Linux 的 `bind mount`功能。

`bind mount` 可以理解为一种目录的共享，它将 dest 目录所指向的 inode 更改为 source 目录所指向的 inode，这样一来，对 dest 目录下的操作就会直接影响到 source 目录。

Docker 命令行里 -v 参数的格式为：`-v source:dest`，其中 source 是宿主机的目录，dest 是容器内的目录，因此我们只需要找到 dest 在宿主机上的路径，然后使用 `bind mount` 将 source 和 dest 目录关联起来即可。

## 2. 具体实现

我们首先需要修改容器 Config，为其添加 `Mounts` 字段，用于存储 volume 的挂载信息。

### libcontainer/config/config.go

```go
type Config struct {
	// 省略...

	// 容器与宿主机的挂载
	Mounts []*Mount `json:"mounts"`

    // 省略...
}
```

`Mount` 结构体定义如下：

### libcontainer/config/mount.go

```go
// Mount 挂载配置
type Mount struct {
	// 源路径，在宿主机上的绝对路径
	Source string `json:"source"`

	// 目标路径，在容器内的绝对路径
	Destination string `json:"destination"`
}
```

之后我们为 `run` 命令添加 `-v` 参数：

### cmd/run.go

```go
var RunCommand = cli.Command{
    // 省略...
	Flags: []cli.Flag{
		// 省略...

		cli.StringSliceFlag{	// 支持多个 -v 参数
			Name:  "v", // 挂载目录
			Usage: "bind mount a volume.	eg: -v /host:/container",
		},
	},

	Action: func(context *cli.Context) error {
		// 生成容器的配置信息
		conf, err := config.CreateConfig(context)
		if err != nil {
			return fmt.Errorf("create config error: %v", err)
		}

		run(conf)

		return nil
	},
}
```

注意我们给 `-v` 参数设置的是 `cli.StringSliceFlag` 类型，这样用户可以传入多个 `-v` 参数，每个 `-v` 参数对应一个 volume 的挂载。

接着我们在 `config.CreateConfig` 函数中解析 `-v` 参数：

### libcontainer/config/utils.go

```go
// 生成容器的 Config 配置
func CreateConfig(ctx *cli.Context) (*Config, error) {
	// 省略...

	// 获取容器的 volume 挂载信息
	mounts, err := extractVolumeMounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract volume mounts: %v", err)
	}

	// 省略...

	return &Config{
        // 省略...
		Mounts:      mounts,
        // 省略...
	}, nil
}

// 解析挂载目录
func extractVolumeMounts(ctx *cli.Context) ([]*Mount, error) {
	var mounts []*Mount

	volumes := ctx.StringSlice("v")
	for _, volume := range volumes {
		volumeArray := strings.Split(volume, ":")
		if len(volumeArray) != 2 {
			return nil, fmt.Errorf("invalid volume: [%v], must split by ':'", volume)
		}

		src, dest := volumeArray[0], volumeArray[1]
		if src == "" || dest == "" {
			return nil, fmt.Errorf("invalid volume: [%v], path can not be empty", volume)
		}

		mounts = append(mounts, &Mount{
			Source:      src,
			Destination: dest,
		})
	}

	return mounts, nil
}
```

解析的逻辑很简单，我们只需要将 `-v` 参数按照 `:` 分割，然后将分割后的两个部分分别作为 `Mount` 结构体的 `Source` 和 `Destination` 字段即可。

由于 `extractVolumeMounts` 方法会返回 `error`，因此我们要同时修改 `CreateConfig` 方法的返回值，将 `error` 传递给上层。

之后我们便可以开始实现 volume 挂载的逻辑了。

### libcontainer/volumes.go

```go
// 将所有指定的 volume 挂载到容器的相应挂载点上
func MountVolumes(conf *config.Config) error {
	mounts := conf.Mounts
	for _, mount := range mounts {
		destInHost := path.Join(conf.Rootfs, mount.Destination)
		if err := mountVolume(mount.Source, destInHost); err != nil {
			return fmt.Errorf("failed to mount volume [%v:%v]: %v", mount.Source, destInHost, err)
		}
		log.Debugf("mount volume [%v:%v] success", mount.Source, destInHost)
	}

	return nil
}

// 使用 bind mount 挂载 volume
func mountVolume(srcInHost string, destInHost string) error {
	// 创建宿主机上的 src 目录
	if err := os.MkdirAll(srcInHost, 0777); err != nil {
		return fmt.Errorf("failed to create src dir: %v", err)
	}
	// 创建宿主机上的 dest 目录
	if err := os.MkdirAll(destInHost, 0777); err != nil {
		return fmt.Errorf("failed to create dest dir: %v", err)
	}

	// 通过 mount 系统调用进行 bind mount
	if err := syscall.Mount(srcInHost, destInHost, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to bind mount: %v", err)
	}

	return nil
}
```

其中 `MountVolumes` 方法暴露给外部调用，它会遍历所有的 `Mount` 对象，然后调用 `mountVolume` 方法进行挂载。`mountVolume` 方法则会先创建宿主机上的 `src` 和 `dest` 目录，然后通过 `mount` 系统调用进行 bind mount。

除此之外，我们还需要封装一个 `UmountVolumes` 方法，用于卸载 volume：

```go
// 卸载容器的所有 volume
func UmountVolumes(conf *config.Config) {
	mounts := conf.Mounts
	for _, mount := range mounts {
		destInHost := path.Join(conf.Rootfs, mount.Destination)
		syscall.Unmount(destInHost, 0)
	}
}
```

这样我们就完成了 volume 挂载的实现，只需要在容器的 `Create` 方法内调用 `MountVolumes` 方法，在 `Remove` 方法内调用 `UmountVolumes` 方法即可：

### cmd/run.go

```go
func (c *Container) Create() error {
	// 省略...

	// 挂载 volume
	if err := MountVolumes(c.Config); err != nil {
		return fmt.Errorf("failed to mount volumes: %v", err)
	}
	
	// 省略...
}

func (c *Container) Remove() {
	// 省略...

	// 卸载 volume
	UmountVolumes(c.Config)

	// 省略...
}
```

## 3. 测试

### 挂载不存在的目录

我们首先实验一下，能否把宿主机不在的目录挂载到容器中。

这里我们选择宿主机的 `/root/no-exist` 目录：

```bash
# sudo ./m-docker run -it -v /root/no-exist:/data
root@master-58:/#
```

打开一个新的终端，查看宿主机的 `/root` 目录：

```bash
# sudo ls /root
no-exist  sources.list  go  snap
```

没错，有 `no-exist` 目录。

现在我们回到容器进程，往 `/data` 目录写入一个文件：

```bash
# echo "hello world" > /data/hello.txt

# ls /data
hello.txt

# cat /data/hello.txt
hello world
```

之后我们查看宿主机的 `/root/no-exist` 目录：

```bash
# sudo ls /root/no-exist
hello.txt

# sudo cat /root/no-exist/hello.txt
hello world
```

文件确实在。

之后我们退出容器，看看数据是否能持久化。

退出容器：
```bash	
# exit
```

重新查看宿主机的 `/root/no-exist` 目录：

```bash
# sudo ls /root/no-exist
hello.txt

# sudo cat /root/no-exist/hello.txt
hello world
```

还在，数据持久化成功。

### 挂载已存在的目录

这次我们挂载一个已经存在的目录。

方便起见，这里就把刚才创建的 `/root/no-exist` 目录再挂载一次。

```bash
# sudo ./m-docker run -it -v /root/no-exist:/data
root@master-58:/#
```

我们把文件内容更新一下，然后退出：

```bash
# echo "m-docker respect to docker" > /data/hello.txt
# cat /data/hello.txt
m-docker respect to docker

# exit
```

在宿主机上查看：

```bash
# sudo cat /root/no-exist/hello.txt
m-docker respect to docker
```

至此，说明我们的 volume 挂载功能完全正常。