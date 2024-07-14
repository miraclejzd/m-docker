package v2

import "m-docker/libcontainer/cgroup/resource"

// cgroup v2 的 controller 抽象接口
type Controller interface {
	// Name() 方法返回当前 Controller 的名字，如 cpu、memory
	Name() string

	// Set() 方法用于设置当前 Controller 的资源限制
	Set(cgroupPath string, resConf *resource.ResourceConfig) error
}

// 所有的 cgroup controller
var Controllers = []Controller{
	&CpuController{},
	&MemoryController{},
}
