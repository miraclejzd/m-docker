package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
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

	// m-docker 数据的根目录
	rootPath = "/var/lib/m-docker"

	// m-docker 状态信息的根目录
	statePath = "/run/m-docker"

	// 容器 Config 文件名
	configName = "config.json"
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
		Rootfs:      path.Join(rootPath, "rootfs", containerID),
		RwLayer:     path.Join(rootPath, "layers", containerID),
		StateDir:    path.Join(statePath, containerID),
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

// 将容器的 Config 持久化存储到磁盘上
func RecordContainerConfig(conf *Config) error {
	// 创建容器的状态信息目录
	if err := os.MkdirAll(conf.StateDir, 0777); err != nil {
		return fmt.Errorf("failed to create container state dir:  %v", err)
	}

	// 将容器 Config 写入文件
	jsonBytes, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("failed to marshal container config:  %v", err)
	}
	jsonStr := string(jsonBytes)
	filePath := path.Join(conf.StateDir, configName)
	file, err := os.Create(filePath)
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorf("failed to close file %v:  %v", filePath, err)
		}
	}()
	if err != nil {
		return fmt.Errorf("failed to create file %s:  %v", filePath, err)
	}
	if _, err = file.WriteString(jsonStr); err != nil {
		return fmt.Errorf("failed to write container config to file %s:  %v", filePath, err)
	}

	return nil
}

// 删除容器的状态信息
func DeleteContainerState(conf *Config) {
	os.RemoveAll(conf.StateDir)
}
