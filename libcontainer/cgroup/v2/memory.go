package v2

import (
	"fmt"
	"m-docker/libcontainer/cgroup/resource"
	"os"
	"path"

	log "github.com/sirupsen/logrus"
)

type MemoryController struct {
}

func (s *MemoryController) Name() string {
	return "memory"
}

func (s *MemoryController) Set(cgroupPath string, resConf *resource.ResourceConfig) error {
	var memLimit string
	if resConf.MemoryLimit == "" { // 如果没有设置内存限制，则默认为最大值
		memLimit = "max"
	} else { // 否则按照设置的值进行限制
		memLimit = resConf.MemoryLimit
	}

	// 将内存限制写入 memory.max 文件
	if err := os.WriteFile(path.Join(cgroupPath, "memory.max"), []byte(memLimit), 0644); err != nil {
		return fmt.Errorf("os.WriteFile() to file %v fail:  %v", path.Join(cgroupPath, "memory.max"), err)
	}

	log.Infof("Set cgroup memory.max: %v", memLimit)
	return nil
}
