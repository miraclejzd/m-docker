package v2

import (
	"fmt"
	"m-docker/libcontainer/config"
	"os"
	"path"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

type CgroupV2Manager struct {
	dirPath     string
	resource    *config.Resources
	controllers []Controller
}

const unifiedMountPoint = "/sys/fs/cgroup"

func NewCgroupV2Manager(dirPath string) *CgroupV2Manager {
	if !strings.HasPrefix(dirPath, unifiedMountPoint) {
		dirPath = path.Join(unifiedMountPoint, dirPath)
	}

	return &CgroupV2Manager{
		dirPath:     dirPath,
		controllers: Controllers,
	}
}

func (c *CgroupV2Manager) Init() error {
	_, err := os.Stat(c.dirPath)
	if err != nil && os.IsNotExist(err) { // 如果 cgroup 目录不存在，则创建
		err := os.Mkdir(c.dirPath, 0755)
		if err != nil {
			return fmt.Errorf("create cgroup dir \"%v\" fail: %v", c.dirPath, err)
		}
	} else { // 如果 cgroup 目录已经存在，则返回错误
		return fmt.Errorf("cgroup dir %s already exists", c.dirPath)
	}
	return nil
}

func (c *CgroupV2Manager) Apply(pid int) error {
	// 将进程的 PID 写入 cgroup.procs 文件
	if err := os.WriteFile(path.Join(c.dirPath, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("os.WriteFile() to file %v fail: %v", path.Join(c.dirPath, "cgroup.procs"), err)
	}

	return nil
}

func (c *CgroupV2Manager) Set(resConf *config.Resources) {
	c.resource = resConf
	// 遍历所有的 cgroup controller，调用 controller 的 Set 方法来设置 cgroup 的资源限制
	for _, controller := range c.controllers {
		if err := controller.Set(c.dirPath, resConf); err != nil {
			log.Warnf("set cgroup controller %v  fail: %v", controller.Name(), err)
		}
	}
}

func (c *CgroupV2Manager) Destroy() {
	os.RemoveAll(c.dirPath)
	os.Remove(c.dirPath)
}
