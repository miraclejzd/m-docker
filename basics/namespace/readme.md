# namespace

第一个底层技术是 namespace.


## 1. 什么是 namespace

namespace（ns） 是 Linux 内核提供的一种轻量级虚拟化技术，它可以将一组进程隔离在一个独立的环境中，使得这组进程**看到的**(这个词很关键)系统资源是独立的。

这么说太官方了，通俗一点是这样：在 Linux 系统中，每个进程其实会被设置许多以 namespace 结尾的属性，例如 PID namespace。假设进程 A 的 PID namespace 为 1，进程 B 的 PID namespace 为 2，那么进程 A 和进程 B 就会看到不同的进程树（显然，进程 A 肯定看不到进程 B）。这样，进程之间就好像完全感受不到其它进程的存在一样。

隔离，这便是 namespace 的核心作用。

我们归纳一下要点：
- 作用： 隔离
- 对谁起作用：进程

### 1.1 查看进程所属的 namespace

Linux 中，每个进程都有 `/proc/[pid]/ns` 这样一个目录，这里包含了这个进程所属的 namespace 的信息：

```bash
# 查看当前 bash 进程的 ns 信息
jzd@master-58:~$ echo $$ && ls -l /proc/$$/ns
3189630  # 当前进程 pid
total 0
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:45 cgroup -> 'cgroup:[4026531835]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:45 ipc -> 'ipc:[4026531839]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:45 mnt -> 'mnt:[4026531841]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:45 net -> 'net:[4026531840]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:45 pid -> 'pid:[4026531836]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:45 pid_for_children -> 'pid:[4026531836]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:45 time -> 'time:[4026531834]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:45 time_for_children -> 'time:[4026531834]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:45 user -> 'user:[4026531837]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:45 uts -> 'uts:[4026531838]'
```
以 `pid:[4026531836]` 为例，其中 `pid` 为 namespace 的类型，`4026531836` 是 inode 编号。

如果当前进程创建子进程，子进程默认会继承父进程的 namespace：
    
```bash
# 产看子 bash 进程的 ns 信息
jzd@master-58:~$ bash -c 'echo $$ && ls -l /proc/$$/ns'
3195573  # 子 bash 进程 pid
total 0
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:55 cgroup -> 'cgroup:[4026531835]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:55 ipc -> 'ipc:[4026531839]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:55 mnt -> 'mnt:[4026531841]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:55 net -> 'net:[4026531840]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:55 pid -> 'pid:[4026531836]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:55 pid_for_children -> 'pid:[4026531836]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:55 time -> 'time:[4026531834]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:55 time_for_children -> 'time:[4026531834]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:55 user -> 'user:[4026531837]'
lrwxrwxrwx 1 jzd jzd 0 Jun  1 13:55 uts -> 'uts:[4026531838]'
```

我们可以发现子进程 `pid` namespace 的 inode 编号和父进程一样，均为 `4026531836`。

### 1.2. namespace 数量限制

当然，Linux 也限制了 namespace 的数量，总不能无限制地创建 namespace 吧。对于各个 ns 的限制在 `/proc/sys/user` 目录中：

```bash
jzd@master-58:~$ tree /proc/sys/user
/proc/sys/user
├── max_cgroup_namespaces
├── max_fanotify_groups
├── max_fanotify_marks
├── max_inotify_instances
├── max_inotify_watches
├── max_ipc_namespaces
├── max_mnt_namespaces
├── max_net_namespaces
├── max_pid_namespaces
├── max_time_namespaces
├── max_user_namespaces
└── max_uts_namespaces

0 directories, 12 files
jzd@master-58:~$ cat /proc/sys/user/max_pid_namespaces
63215
```

可以看出，当前这个 Linux 系统限制了只能有 `63215` 个 PID namespace。


## 2. Golang 操作 namespace 实操

源代码可见 [这里](./src/main.go)。

### PID namespace

我们写一个程序，创建一个 bash 子进程，并为这个子进程创建新的 PID namespace：

```go
// 注: 需要 root 权限。
func main() {
	cmd := exec.Command("bash")
	cmd.SysProcAttr = &syscall.SysProcAttr{
        // 创建新的 PID namespace
		Cloneflags: syscall.CLONE_NEWPID, 
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalln(err)
	}
}
```

运行并测试：

```bash
# 编译并运行
jzd@master-58:~/projects/m-docker/basics/namespace/src$ go build && sudo ./namespace

# 查看 namespace 进程的 pid
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src$ ps | grep namespace
3227671 pts/31   00:00:00 namespace

# 查看 bash 子进程的 pid
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src$ echo $$
1

# 查看 namespace 进程的进程树
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src$ pstree -pl 3227671
namespace(3227671)─┬─bash(3227676)───pstree(3228424)
                   ├─{namespace}(3227672)
                   ├─{namespace}(3227673)
                   ├─{namespace}(3227674)
                   └─{namespace}(3227675)
```

我们可以发现，namespace 进程的 pid 为 `3227671`，使用 `$$` 看到 bash 子进程的 pid 为`1`，确实符合父子进程 PID namespace 不一样的结论。但是使用 `pstree` 查看进程树，我们发现 bash 子进程的 pid 却是 `3227676`，蛮奇怪的。

我查了查，pstree 会访问 `/proc` 目录来进行进程树的构建，而 `/proc` 目录的内容实际上是被挂载在这里的 **proc 文件系统**。我们的 bash 子进程只设置了新的 PID namespace，Mount namespace 并没有被设置，导致子进程的所有挂载点会完全继承父进程的，所以 `/proc` 目录的内容会和父进程的一样。


那我们干脆就在父进程的 `/proc` 里，查看它们的 PID namespace:

```bash
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src$ readlink /proc/3227671/ns/pid
pid:[4026531836]
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src$ readlink /proc/3227676/ns/pid
pid:[4026532392]
```

果然，子进程和父进程其实拥有了不同的 PID namespace。

那我们把子进程的 `/proc` 挂载成自己的 proc 文件系统，看看会发生什么：

```bash
# 重新挂载 proc 文件系统
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src$ mount -t proc myproc /proc

# 查看当前 PID ns 下所有的进程
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src# ps -a
    PID TTY          TIME CMD
      1 pts/28   00:00:00 bash
    137 pts/28   00:00:00 ps
```

太对啦！现在 proc 文件系统只显示当前 PID ns 下的进程，连自己老爹都不认识了。

### Mount namespace

上面对 PID ns 的操作中刚好提到了 Mount ns，我们趁热打铁，来看看 Mount ns 的操作吧。

讲个冷知识，Mount ns 是 Linux 的第一个 namespace，地位相当重要，它用来隔离各个进程的挂载点视图。众所周知，Linux 的挂载功能十分的强大，我们可以通过挂载来改变目录树的层级结构，就像这样：

```
     目录树(当前为rootfs)     tmpfs                             目录树(rootfs+tmpfs)
	    /                 /            mount tmpfs /var         /
	    ├── home          ├── file1    ───────────────>         ├── home
	    ├── usr           └── file2                             ├── usr
	    └── var                                                 └── var 
                                                                         ├── file1
                                                                         └── file2
```

而 Mount ns 的作用就是让不同的进程能看到的挂载点不一样，这样就可以实现文件系统的隔离。

我们修改一下代码，创建新的 Mount namespace：

```go
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// 创建新的 PID 和 Mount Namespace。
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}
```

运行并测试：

```bash
# 编译并运行
jzd@master-58:~/projects/m-docker/basics/namespace/src$ go build && sudo ./namespace

# 查看 /proc
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src$ ls /proc
124  140      18      193495  226023  252     265663   280379   2840761  286094   3        32       3853953  403  501     539911  580493  598     656729  67489   686420  71792   786872  885 ...
```

咦？怎么设置了新的 Mount ns 后，挂载点还和父进程的一样？

查阅了之后才明白：即使设置了新的 Mount ns，子进程依旧会继承父进程的所有挂载点，并不会将全部挂载点清空（不然的话根文件系统都不会挂载在目录树上，我们的目录树应该是空的），只是后续在子进程进行挂载操作，不会影响到父进程。

原来如此，那我们在子进程挂载重新挂载 proc 文件系统 ，看看会不会影响到父进程：

```bash
# 子进程重新挂载 proc 文件系统
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src# mount -t proc proc /proc

# 子进程查看 /proc
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src$ ls /proc
1     asound      bus      consoles  devices    driver         fb           interrupts  irq       keys       kpagecgroup  loadavg  meminfo  mounts  pagetypeinfo  schedstat  slabinfo  swaps ...
```

在宿主机上重新打开一个 bash 终端（和父进程同级，我称为叔叔进程）：
```bash
# 叔叔进程查看 /proc
jzd@master-58:~$ ls /proc
1     asound      bus      consoles  devices    driver         fb           interrupts  irq       keys       kpagecgroup  loadavg  meminfo  mounts  pagetypeinfo  schedstat  slabinfo  swaps ...
```

怎么回事？子进程的 proc 文件系统重新挂载后，叔叔进程的 proc 文件系统也被影响了？Mount ns 没起到作用呀！

我们先解除子进程的 proc 文件系统挂载，免得影响宿主机的使用：

```bash
# 在子进程上 umount
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src# umount proc
```

又查阅了一些资料后才明白：Linux 具有 `shared subtree` 机制，这个机制的优先级高于 Mount ns。

简单来说 `shared subtree` 机制有 2 个条件：
- 两个挂载点属于同一个 `peer group`
- 他们的 `propagate type` 均为 `shared`
  
满足这 2 个条件的话，这两个挂载点所在的目录，以及它所有子目录里任何挂载操作都会被同步。

我们看看父子进程是否满足这 2 个条件：
- 子进程会完全继承父进程的所有挂载点，合理推断它们的确在同一个 `peer group`，满足。
- 子进程会继承父进程的所有挂载点，那我们只要看父进程的 `/proc` 挂载点是否为 `shared` 类型即可，待验证。

验证一下：
```bash
# 查看叔叔进程的 /proc 挂载点类型
jzd@master-58:~$ cat /proc/self/mountinfo | grep /proc | grep shared
25 30 0:23 / /proc rw,nosuid,nodev,noexec,relatime shared:59 - proc proc rw
38 25 0:33 / /proc/sys/fs/binfmt_misc rw,relatime shared:61 - autofs systemd-1 rw,fd=29,pgrp=1,timeout=0,minproto=5,maxproto=5,direct,pipe_ino=22032
116 38 0:37 / /proc/sys/fs/binfmt_misc rw,nosuid,nodev,noexec,relatime shared:99 - binfmt_misc binfmt_misc rw
```

果然啊，父进程的 `/proc` 挂载点类型为 `shared`，所以子进程继承之后也为 `shared`，确实会同步！如何解决呢？

很简单，我们只需要让子进程的 `/proc` 目录不为 `shared` 即可，查阅资料后发现，`propagate type` 可以设置为 `private`，这样就不会同步了：

```bash
# 子进程递归地将 /proc 及其子目录设置为 private
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src# mount --make-rprivate /proc

# 重新挂载 proc 文件系统并查看
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src# mount -t proc proc /proc
root@master-58:/home/jzd/projects/m-docker/basics/namespace/src# ls /proc
1     asound      bus      consoles  devices    driver         fb           interrupts  irq       keys
```

我们打开叔叔进程，看看会不会有影响：
```bash
# 叔叔进程查看 /proc
jzd@master-58:~$ ls /proc
1    124  140      18      193495  230546  253     267940   2840759  295315  301133   30564   311062  323 ...
```

太对啦！叔叔进程的 proc 文件系统没有被影响，Mount ns 真的起作用了。

