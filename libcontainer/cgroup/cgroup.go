package cgroup

import (
	"fmt"
	"m-docker/libcontainer/cgroup/resource"
	v2 "m-docker/libcontainer/cgroup/v2"

	log "github.com/sirupsen/logrus"
)

// Cgroup 是 cgroup 的抽象接口
type Cgroup interface {
	// 初始化 cgroup，创建 cgroup 目录
	Init() error

	// 将进程 pid 添加至 cgroup 中
	Apply(pid int) error

	// 设置 cgroup 的资源限制
	Set(res *resource.ResourceConfig)

	// 销毁 cgroup
	Destroy()
}

// 根据 cgroup 版本创建 CgroupManager
func NewCgroupManager(dirPath string) (Cgroup, error) {
	// 如果支持 cgroup v2，则使用 cgroup v2
	if IsCgroup2UnifiedMode() {
		log.Infof("using cgroup v2")
		return v2.NewCgroupV2Manager(dirPath), nil
	}
	// 目前不考虑支持 cgroup v1，因此直接返回错误
	return nil, fmt.Errorf("cgroup v2 is not supported")
}
