package v2

import (
	"fmt"
	"m-docker/libcontainer/config"
	"os"
	"path"

	log "github.com/sirupsen/logrus"
)

type MemoryController struct {
}

func (s *MemoryController) Name() string {
	return "memory"
}

func (s *MemoryController) Set(cgroupPath string, resConf *config.Resources) error {
	// 将内存限制写入 memory.max 文件
	if err := os.WriteFile(path.Join(cgroupPath, "memory.max"), []byte(resConf.Memory), 0644); err != nil {
		return fmt.Errorf("os.WriteFile() to file %v fail:  %v", path.Join(cgroupPath, "memory.max"), err)
	}

	log.Debugf("Set cgroup memory.max: %v", resConf.Memory)
	return nil
}
