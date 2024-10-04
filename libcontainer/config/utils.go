package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"m-docker/libcontainer/constant"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/exp/rand"
)

const (
	// 默认 CPU 硬限制调度周期为 100000us
	defaultCPUPeriod = 100000
)

// 生成容器的 Config 配置
func CreateConfig(ctx *cli.Context) (*Config, error) {
	// 容器创建时间
	utcPlus8 := time.FixedZone("UTC+8", 8*60*60)
	createdTime := time.Now().In(utcPlus8).Format("2006-01-02 15:04:05")

	// 从命令行参数中获取容器名称
	containerName := ctx.String("name")
	// 如果没有设置容器名称，则生成一个随机名称
	if containerName == "" {
		containerName = generateContainerName()
	}

	// 生成容器ID
	containerID := generateContainerID(containerName + createdTime)

	// 获取容器的 volume 挂载信息
	mounts, err := extractVolumeMounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract volume mounts: %v", err)
	}

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

	// 判断容器在前台运行还是后台运行
	tty := ctx.Bool("it")
	detach := ctx.Bool("detach")
	if tty && detach { // 特判同时设置 -it 和 -d 的情况
		return nil, fmt.Errorf("it and detach can not be set at the same time")
	}
	// 这里并不需要判断 detach 为 true 的情况，因为 detach 为 true 时，tty 必为 false

	return &Config{
		ID:          containerID,
		Name:        containerName,
		Rootfs:      path.Join(constant.RootPath, "rootfs", containerID),
		RwLayer:     path.Join(constant.RootPath, "layers", containerID),
		StateDir:    path.Join(constant.StatePath, containerID),
		LogPath:     path.Join(constant.StatePath, containerID, constant.LogFileName),
		Mounts:      mounts,
		TTY:         tty,
		CmdArray:    cmdArray,
		Cgroup:      createCgroupConfig(ctx, containerID),
		CreatedTime: createdTime,
	}, nil
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

// 生成 cgroup 配置
func createCgroupConfig(ctx *cli.Context, containerID string) *Cgroup {
	name := "m-docker-" + containerID

	return &Cgroup{
		Name:      name,
		Path:      path.Join(constant.CgroupRootPath, name+".scope"),
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
	filePath := path.Join(conf.StateDir, constant.ConfigName)
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

// 根据容器状态目录路径获取容器 Config
func GetConfigFromStatePath(statePath string) (*Config, error) {
	configPath := path.Join(statePath, constant.ConfigName)
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", configPath, err)
	}

	conf := new(Config)
	if json.Unmarshal(content, conf) != nil {
		return nil, fmt.Errorf("failed to unmarshal json content: %v", err)
	}
	return conf, nil
}

// 根据容器 ID 获取容器 Config
func GetConfigFromID(id string) (*Config, error) {
	// 获取容器完整 ID
	fullID, err := GetIDFromPrefix(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get container ID: %v", err)
	}

	statePath := path.Join(constant.StatePath, fullID)
	return GetConfigFromStatePath(statePath)
}

// 根据容器 ID 的前缀还原完整的容器 ID
func GetIDFromPrefix(id string) (string, error) {
	files, err := os.ReadDir(constant.StatePath)
	if err != nil {
		return "", fmt.Errorf("read dir %s error: %v", constant.StatePath, err)
	}
	// 遍历所有容器状态目录，找到 ID 前缀匹配的容器
	for _, file := range files {
		if strings.HasPrefix(file.Name(), id) {
			return file.Name(), nil
		}
	}
	return "", fmt.Errorf("container \"%s\" not found", id)
}

// 从容器名称获取容器 ID
func GetIDFromName(name string) (string, error) {
	dirs, err := os.ReadDir(constant.StatePath)
	if err != nil {
		return "", fmt.Errorf("read dir %s error: %v", constant.StatePath, err)
	}

	for _, dir := range dirs {
		statePath := path.Join(constant.StatePath, dir.Name())
		conf, err := GetConfigFromStatePath(statePath)
		if err != nil {
			return "", fmt.Errorf("get config from state path %s error: %v", dir.Name(), err)
		}

		if conf.Name == name {
			return dir.Name(), nil
		}
	}

	return "", fmt.Errorf("container \"%s\" not found", name)
}

// 从容器名称或 ID 前缀获取容器完整 ID
func GetIDFromNameOrPrefix(nameOrID string) (string, error) {
	id, err := GetIDFromName(nameOrID)
	if err != nil {
		id, err = GetIDFromPrefix(nameOrID)
		if err != nil {
			return "", fmt.Errorf("container %s not found", nameOrID)
		}
	}

	return id, nil
}
