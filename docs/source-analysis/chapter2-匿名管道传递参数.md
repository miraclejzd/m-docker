# chapter2 - 优化：匿名管道传递参数

目前我们已经实现了一个简单的 run 命令，但是它传递参数的方式比较简单直接，就像这样：

```go
args := []string{"init", command}
cmd := exec.Command("/proc/self/exe", args...)
```

所有的参数会被直接传递给子进程，并直接作为命令行参数被解析。

这种方式存在着很大的问题：**如果用户的输入特别长（超过命令行参数的长度限制），或是里面有特殊字符，就会导致解析错误**。

这一节我们进行改造，使用 runc 同款的**匿名管道**进行父子进程之间的参数传递。

## 什么是匿名管道

匿名管道是一种特殊的**文件描述符**，用于在**有亲缘关系**的进程之间进行通信（两个无关进程之间不能用匿名管道，要用命名管道）

匿名管道有以下特点：
- 虽然是文件描述符，但并不与磁盘交互，而是在内存中（被称为内存缓冲区）传递数据
- 缓冲区大小固定，一般是 4KB
- 单向通信，一端写入，一端读取
- 管道写满时，写入端会阻塞，直到读取端读取数据
- 管道空着时，读取端会阻塞，直到写入端写入数据

## 具体实现

在运行 `m-docker run` 的进程里创建一个匿名管道，并将写入端给父进程，读取端给子进程。

### container-process.go

我们改写 `NewContainerProcess` 函数，在创建容器进程句柄的同时创建匿名管道。

```go
func NewContainerProcess(tty bool) (*exec.Cmd, *os.File) {
	// 创建一个匿名管道用于传递参数，readPipe 和 writePipe 分别传递给子进程和父进程
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		log.Errorf("New pipe error: %v", err)
		return nil, nil
	}

	cmd := exec.Command("/proc/self/exe", "init")

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}

	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	// 将 readPipe 通过子进程的 cmd.ExtraFile 传递给子进程
	cmd.ExtraFiles = []*os.File{readPipe}

	return cmd, writePipe
}
```

重点是这一句：

```go
cmd.ExtraFiles = []*os.File{readPipe}
```

将 `readPipe` 作为 `cmd` 的 `ExtraFiles`，这样 cmd 这个句柄就会带着 `readPipe` 这个文件描述符去创建子进程。

创建好了管道，接下来就要考虑数据的读写了。

### run.go

父进程在创建完管道之后天然就拿到了 `writePipe`，我们只需要选择合适的时候将数据写入管道即可。

什么时候比较合适呢？

答案是：**子进程创建之后**。

试想一下，如果在子进程创建之前就写入数据，假如这个用户素质很低，输入了一个很长的命令（远超缓冲区的大小），那么就会导致管道写满，父进程阻塞。而解除阻塞的方式是：管道被子进程读取数据。那么问题来了，子进程还没创建，怎么读取数据呢？

很巧妙啊。

```go
func run(tty bool, command string) {
	process, writePipe := libcontainer.NewContainerProcess(tty)
	if process == nil {
		log.Errorf("New process error!")
		return
	}

    // 启动容器进程
	if err := process.Start(); err != nil {
		log.Errorf("Run process.Start() err: %v", err)
	}
	// 子进程创建之后再通过管道发送参数
	sendInitCommand(command, writePipe)

	_ = process.Wait()
	os.Exit(-1)
}
```

来看看 `sendInitCommand` 函数：

```go
// 通过匿名管道发送参数给子进程
func sendInitCommand(command string, writePipe *os.File) {
	log.Infof("Send command to init: %s", command)
	_, _ = writePipe.WriteString(command)
	_ = writePipe.Close()
}
```

很简单，就是一个很简单的文件描述符的写入操作。

父进程写入了数据之后，我们看看子进程是怎么读取的。

### init.go

子进程启动后，首先要找到前面通过 `cmd.ExtraFiles` 传递过来的 `readPipe` ，然后才是数据的读取。

```go
const readPipefdIndex = 3

func readPipeCommand() []string {
	// uintPtr(3) 就是指 index 为 3 的文件描述符，至于为什么是3，具体解释一下：
	// 每个进程在创建的时候默认有3个文件描述符，分别是：
	// 0: 标准输入
	// 1: 标准输出
	// 2: 标准错误
	// 我们在之前创建 cmd 时设置了 cmd.ExtraFiles = []*os.File{readPipe}
	// 因此这里的 index 就是3
	pipe := os.NewFile(uintptr(readPipefdIndex), "pipe")

    // 读取管道的数据
	msg, err := io.ReadAll(pipe)
	if err != nil {
		log.Errorf("read pipe error: %v", err)
		return nil
	}

	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}
```

子进程拿到数据之后就可以开始运行命令啦：

```go
func initContainer() error {
	log.Infof("Start func: initContainer")

	// 挂载 proc 文件系统
	mountProcFS()

	// 读取管道中的 command 参数
	cmdArray := readPipeCommand()
	if len(cmdArray) == 0 {
		return errors.New("get user command error, cmdArray is nil")
	}

	// 判断用户指定的 command 的可执行文件路径是否存在
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

这里的代码相对于 `feat-run` 进行了一点重构，但是整体逻辑没有变，运行的流程是一样的。
