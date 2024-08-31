package cmd

import (
	"fmt"
	"m-docker/libcontainer/config"
	"m-docker/libcontainer/constant"
	"os"
	"strings"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// m-docker ps 命令
var ContainerListCommand = cli.Command{
	Name:      "ps",
	Usage:     `show all the containers in list`,
	UsageText: `m-docker ps`,

	Action: func(context *cli.Context) error {
		if err := listContainers(); err != nil {
			return fmt.Errorf("list containers error: %v", err)
		}
		return nil
	},
}

// 查询 m-docker 状态目录下的所有目录，根据 config.json 文件获取容器信息
func listContainers() error {
	// 读取状态目录下的所有容器目录
	files, err := os.ReadDir(constant.StatePath)
	if err != nil {
		return fmt.Errorf("read dir %s error: %v", constant.StatePath, err)
	}

	// 遍历所有容器目录，获取容器 Config
	containersConfigs := make([]*config.Config, 0, len(files))
	for _, file := range files {
		conf, err := config.GetConfigFromID(file.Name())
		if err != nil {
			log.Warningf("get config from id %s error: %v", file.Name(), err)
			continue
		}
		containersConfigs = append(containersConfigs, conf)
	}

	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	_, err = fmt.Fprintf(w, "CONTAINER ID\tPID\tCOMMAND\tCREATED\tSTATUS\tNAME\n")
	if err != nil {
		return fmt.Errorf("failed to execute fmt.Fprintf: %v", err)
	}
	for _, item := range containersConfigs {
		_, err = fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\n",
			item.ID[:12],
			item.Pid,
			strings.Join(item.CmdArray, " "),
			item.CreatedTime,
			item.Status,
			item.Name,
		)
		if err != nil {
			return fmt.Errorf("failed to execute fmt.Fprintf: %v", err)
		}
	}
	w.Flush()

	return nil
}
