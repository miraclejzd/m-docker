package cmd

import (
	"fmt"
	"m-docker/libcontainer/config"
	"os"

	"github.com/urfave/cli"
)

// m-docker logs 命令
var LogsCommand = cli.Command{
	Name:      "logs",
	Usage:     `fetch the logs of a container`,
	UsageText: `m-docker logs CONTAINER`,

	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("\"m-docker run\" requires at least 1 argument")
		}

		// 获取容器 ID
		c := context.Args().Get(0)
		id, err := config.GetIDFromName(c)
		if err != nil {
			id, err = config.GetIDFromPrefix(c)
			if err != nil {
				return fmt.Errorf("container %s not found", c)
			}
		}

		// 打印容器日志
		err = logContainer(id)
		if err != nil {
			return fmt.Errorf("failed to log container %s: %v", id, err)
		}

		return nil
	},
}

// 查询容器的日志文件，并打印
func logContainer(id string) error {
	// 获取容器 Config
	conf, err := config.GetConfigFromID(id)
	if err != nil {
		return fmt.Errorf("failed to get container config: %v", err)
	}

	// 读取容器日志文件
	logContent, err := os.ReadFile(conf.LogPath)
	if err != nil {
		return fmt.Errorf("failed to read log file: %v", err)
	}

	// 打印日志内容至标准输出
	_, err = fmt.Fprint(os.Stdout, string(logContent))
	if err != nil {
		return fmt.Errorf("failed to print log content: %v", err)
	}

	return nil
}
