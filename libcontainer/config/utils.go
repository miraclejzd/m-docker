package config

import (
	"crypto/sha256"
	"fmt"
	"path"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/exp/rand"
)

const (
	// 默认 CPU 硬限制调度周期为 100000us
	defaultCPUPeriod = 100000

	// cgroup 根目录
	cgroupRootPath = "/sys/fs/cgroup/m-docker.slice"
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
		TTY:         ctx.Bool("it"),
		CmdArray:    cmdArray,
		Cgroup:      createCgroupConfig(ctx, containerID),
		CreatedTime: createdTime,
	}
}

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
