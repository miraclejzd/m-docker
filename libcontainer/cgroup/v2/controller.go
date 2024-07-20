package v2

import "m-docker/libcontainer/cgroup/resource"

// cgroup controller 的抽象接口
type Controller interface {
	// Name() 方法返回当前 cgroup controller 的名字，如 cpu、memory
	Name() string

	// Set() 方法用于设置当前 cgroup controller ontroller 的资源限制
	Set(cgroupPath string, resConf *resource.ResourceConfig) error
}

// 所有的 cgroup controller
var Controllers = []Controller{
	&CpuController{},
	&MemoryController{},
}
