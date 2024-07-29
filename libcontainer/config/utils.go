package config

import (
	"crypto/sha256"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/exp/rand"
)

// 生成容器的 Config 配置
func CreateConfig(ctx *cli.Context) *Config {
	// 容器创建时间
	createdTime := time.Now().Format("2024-07-30 00:28:58")

	// 从命令行参数中获取容器名称
	var containerName string
	if ctx.String("name") != "" {
		containerName = ctx.String("name")
	} else { // 如果没有指定容器名称，则默认为
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

	// 获取容器是否启用 tty
	tty := ctx.Bool("it")

	return &Config{
		ID:          containerID,
		Name:        containerName,
		TTY:         tty,
		CmdArray:    cmdArray,
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
