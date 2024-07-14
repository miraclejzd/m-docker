package v2

import (
	"fmt"
	"m-docker/libcontainer/cgroup/resource"
	"os"
	"path"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type CpuController struct {
}

const (
	DefaultPeriod = 100000 // 默认调度周期为 100000us
)

func (s *CpuController) Name() string {
	return "cpu"
}

func (s *CpuController) Set(cgroupPath string, resConf *resource.ResourceConfig) error {
	var cpuLimit string
	if resConf.CpuLimit == 0 { // 如果没有设置 CPU 使用率限制，则默认为最大值
		cpuLimit = "max " + strconv.Itoa(DefaultPeriod)
	} else { // 如果设置了 CPU 使用率限制，则按照设置的值进行限制
		cpuLimit = fmt.Sprintf("%s %v", strconv.Itoa(int(DefaultPeriod*resConf.CpuLimit)), DefaultPeriod)
	}

	// 将 CPU 使用率限制写入 cpu.max 文件
	if err := os.WriteFile(path.Join(cgroupPath, "cpu.max"), []byte(cpuLimit), 0644); err != nil {
		return fmt.Errorf("os.WriteFile() to file %v fail:  %v", path.Join(cgroupPath, "cpu.max"), err)
	}

	log.Infof("Set cgroup cpu.max: %v", cpuLimit)
	return nil
}
