#define _GNU_SOURCE
#include <unistd.h>
#include <errno.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <sys/syscall.h>

char ENV_SETNS_PID[] = "SETNS_PID";

// 打开指定进程的进程文件描述符（pidfd）
static int pidfd_open(pid_t pid, unsigned int flags){
    return syscall(SYS_pidfd_open, pid, flags);
}

// nsenter() 函数将当前进程加入到指定的 namespace 中
void nsenter(){
    char *pid;
    pid = getenv(ENV_SETNS_PID);
    if(pid){
        fprintf(stdout, "got container pid from environment variable %s: %s\n", ENV_SETNS_PID, pid);
    } else {
        // 如果没有设置 MDocker_PID 环境变量，则直接退出
        return;
    }

	int pidfd = pidfd_open(atoi(pid), 0);
	if (pidfd < 0){
		fprintf(stderr, "pidfd_open failed: %s\n", strerror(errno));
		exit(0);
	}

	if (setns(pidfd, CLONE_NEWIPC | CLONE_NEWUTS | CLONE_NEWNET | CLONE_NEWPID | CLONE_NEWNS) != 0) {
		fprintf(stderr, "setns to pid %s failed: %s\n", pid, strerror(errno));
		exit(0);
	} else {
		fprintf(stdout, "setns to pid %s success\n", pid);
	}

	// 由于上面修改了当前进程的 pid ns，原则上对当前进程的 pid ns 修改不会生效，创建的子进程才生效
	// 因此需要 fork 一个子进程来运行，父进程阻塞在这里
	// https://man7.org/linux/man-pages/man2/setns.2.html#description
	// 但实测当前进程的 pid ns 的修改生效了，但继续运行 Go Runtime 会报 pthread_create 的错，可能也与 pid ns 有关
	// 所以还是 fork 一个子进程来运行吧...
	pid_t child_pid = fork();
	if (child_pid < 0) {
		fprintf(stderr, "fork failed: %s\n", strerror(errno));
		exit(EXIT_FAILURE);
	} else if (child_pid > 0) { // 父进程阻塞在这里
		int status;
		if (waitpid(child_pid, &status, 0) == -1) {
			fprintf(stderr, "waitpid failed: %s\n", strerror(errno));
		}
		exit(0);	// 子进程运行结束后，父进程直接退出
	}

	// 子进程跳出 cgo，返回 Go 代码
	return;
}