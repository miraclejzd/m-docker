# chapter4 - 使用 pivot_root 切换根文件系统

前几个章节中，我们已经通过 namespace 和 cgroup 技术创建了一个简单的容器，具有视图隔离和资源限制的功能。

但是大家可以发现，容器内的文件系统还是和宿主机的一样，并不是独立的，这似乎和 Docker 容器的特性不符。

因此本章我们将切换容器的根文件系统（rootfs），实现文件系统的自由。

## 1. 准备根文件系统

即然我们要切换容器的 rootfs，那我们就得提前在宿主机上准备一个精简的文件系统，然后容器运行的时候将这个文件系统挂载为根目录。

这里我拉取了 `ubuntu:latest` 镜像，并解压到了 `/root/rootfs` 目录下，作为未来容器的 rootfs。

```bash
# 拉取 ubuntu 镜像
docker pull ubuntu

# 让 ubuntu 容器在后台持续的运行
docker run -d ubuntu top

# 使用 docker export 命令将正在运行的 ubuntu 容器的文件系统导出为 rootfs.tar 压缩包
docker export <容器ID> -o rootfs.tar 

# 解压 rootfs.tar 到 /root/rootfs 目录下
sudo mkdir /root/rootfs
tar -xvf rootfs.tar -C /root/rootfs/
```

`/root/rootfs` 的内容大概长这样：

```bash
# sudo ls -l /root/rootfs
total 60
lrwxrwxrwx  1 root root    7 Apr 22 13:08 bin -> usr/bin
drwxr-xr-x  2 root root 4096 Apr 22 13:08 boot
drwxr-xr-x  4 root root 4096 Jul 23 08:09 dev
drwxr-xr-x 32 root root 4096 Jul 23 08:09 etc
drwxr-xr-x  3 root root 4096 Jun  5 02:06 home
lrwxrwxrwx  1 root root    7 Apr 22 13:08 lib -> usr/lib
lrwxrwxrwx  1 root root    9 Apr 22 13:08 lib64 -> usr/lib64
drwxr-xr-x  2 root root 4096 Jun  5 02:02 media
drwxr-xr-x  2 root root 4096 Jun  5 02:02 mnt
drwxr-xr-x  2 root root 4096 Jun  5 02:02 opt
drwx------  3 root root 4096 Jul 23 08:15 root
drwxr-xr-x  4 root root 4096 Jun  5 02:06 run
lrwxrwxrwx  1 root root    8 Apr 22 13:08 sbin -> usr/sbin
drwxr-xr-x  2 root root 4096 Jun  5 02:02 srv
drwxr-xr-x  2 root root 4096 Apr 22 13:08 sys
drwxrwxrwt  2 root root 4096 Jun  5 02:05 tmp
drwxr-xr-x 12 root root 4096 Jun  5 02:02 usr
drwxr-xr-x 11 root root 4096 Jun  5 02:05 var
```

可以看到，这和咱们平时接触到的 Linux 的 rootfs 几乎一模一样。

## 2.切换 rootfs 的原理

`pivot_root` 是一个系统调用，主要功能是去切换当前 mount namespace 内所有进程的 rootfs。

`pivot_root` 的原型如下：

```C
int pivot_root(const char *new_root, const char *put_old);
```

- `new_root`：将这个目录作为新的 rootfs
- `put_old`：将当前 rootfs 的内容移动到的这个目录下

调用 `pivot_root` 系统调用后，原先 rootfs 的内容会被转移到 `put_old` 目录下，而 `new_root` 目录会成为新的 rootfs。

> 注意：`new_root` 和 `put_old` 两个目录都必须是挂载点。

**pivot_root 和 chroot 有什么区别？**

-  `pivot_root` 会修改当前 mount namespace 内所有进程的 rootfs
-  `chroot` 只会修改当前进程的 rootfs
  

## 3. 具体实现

挂载 rootfs 的时机应该是容器进程创建之后，在执行 `initContainer` 函数的时候。

### cmd/init.go

```go
unc initContainer() error {
	log.Infof("Start func: initContainer")

	// 挂载根文件系统
	mountRootFS()

	cmdArray := readPipeCommand()
	if len(cmdArray) == 0 {
		return errors.New("get user command error, cmdArray is nil")
	}

	path, err := exec.LookPath(cmdArray[0])
	if err != nil {
		log.Errorf("exec.LookPath error: %v", err)
		return err
	}
	log.Infof("find command path: %s", path)

	if err := syscall.Exec(path, cmdArray, os.Environ()); err != nil {
		log.Errorf("syscall.Exec error: %v", err.Error())
	}

	return nil
}
```

我们将原来的 `mountProcFS` 改名为 `mountRootFS`，挂载 proc 文件系统的逻辑在切换 rootfs 之后再执行。

```go
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

    // 切换根文件系统
	err = pivotRoot(pwd)
	if err != nil {
		log.Errorf("pivotRoot error: %v", err)
		return
	}

	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	_ = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

    // 重新挂载 /dev 文件系统
    // 若不挂载，会导致容器内部无法访问和使用许多设备，这可能导致系统无法正常工作
	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
}
```

`pivotRoot` 函数的实现如下：

```go
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
```

值得关注的是最开头的 `bind mount` 操作，这是因为 `pivot_root` 系统调用要求 `new_root` 和 `put_old` 都是挂载点，而 `newRoot` 可能并不是挂载点，因此我们需要使用 `bind mount` 将其转化为挂载点。

之后 `pivotRoot` 函数会将当前工作目录作为新的 rootfs，因此在创建容器进程句柄时，我们要设置 cwd 为 `/root/rootfs`。

### libcontainer/congtainer-process.go

```go
func NewContainerProcess(tty bool) (*exec.Cmd, *os.File) {
	...

	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	cmd.ExtraFiles = []*os.File{readPipe}

	// 设置容器进程的工作目录为已设置好的 rootfs 目录
	cmd.Dir = "/root/rootfs"

	return cmd, writePipe
}
```

至此 rootfs 的切换就完成了，我们测试一下。

## 4. 测试

测试比较简单，我们 `ls /` 一下，看看 rootfs 是不是真的切换了：

```bash
# sudo ./m-docker run -it /bin/ls
bin  boot  dev  etc  hello.txt  home  lib  lib64  media  mnt  opt  proc  root  run  sbin  srv  sys  tmp  usr  var
```

可以看到，现在打印出来的确实就是 `/root/rootfs` 下的内容，rootfs 切换成功！

我们接下来尝试一下在容器里创建一个新文件：

```bash
# 启动容器
sudo ./m-docker run -it

# 在容器里执行
echo "hello world" > hello.txt
```

之后我们退出容器，在宿主机上查看：

```bash
# sudo cat /root/rootfs/hello.txt
hello world
```

很合理，我们在容器里创建的文件也确实保存在了 `/root/rootfs` 目录下。

不过若是多思考一下，我们会发现这样的实现还是有问题：如果我们希望后续引入**镜像**的概念，那么按照 OCI 镜像规范，镜像层需要是只读的，这样才能实现高效复用。而我们现在的 `/root/rootfs` 目录可以被直接影响，这显然是不对的。

因此我们下一节需要实现镜像的只读层与容器的读写层的叠加。

