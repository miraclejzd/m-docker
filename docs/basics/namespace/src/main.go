package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

// 注: 运行时需要 root 权限。
func main() {
	cmd := exec.Command("bash")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// 创建新的 PID  Namespace。
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalln(err)
	}
}
