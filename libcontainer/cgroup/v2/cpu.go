package v2

import (
	"fmt"
	"m-docker/libcontainer/config"
	"os"
	"path"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type CpuController struct {
}

func (s *CpuController) Name() string {
	return "cpu"
}

func (s *CpuController) Set(cgroupPath string, resConf *config.Resources) error {
	var cpuLimit string
	if resConf.CpuQuota == 0 { // 如果没有设置 CPU 使用率限制，则默认为最大值
		cpuLimit = "max " + strconv.Itoa(int(resConf.CpuPeriod))
	} else { // 如果设置了 CPU 使用率限制，则按照设置的值进行限制
		cpuLimit = fmt.Sprintf("%v %v", resConf.CpuQuota, resConf.CpuPeriod)
	}

	// 将 CPU 使用率限制写入 cpu.max 文件
	if err := os.WriteFile(path.Join(cgroupPath, "cpu.max"), []byte(cpuLimit), 0644); err != nil {
		return fmt.Errorf("os.WriteFile() to file %v fail:  %v", path.Join(cgroupPath, "cpu.max"), err)
	}

	log.Debugf("Set cgroup cpu.max: %v", cpuLimit)
	return nil
}
