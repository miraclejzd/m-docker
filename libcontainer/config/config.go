package config

// 包含了容器的所有配置信息
type Config struct {
	// 容器的运行状态
	Status string `json:"status"`

	// 容器的进程在宿主机上的 PID
	Pid int `json:"pid"`

	// 容器的唯一标识符
	ID string `json:"ID"`

	// 容器名称
	Name string `json:"name"`

	// 容器的 rootfs 路径
	Rootfs string `json:"rootfs"`

	// 容器的读写层路径
	RwLayer string `json:"rwLayer"`

	// 容器的状态信息路径
	StateDir string `json:"stateDir"`

	// 容器与宿主机的挂载
	Mounts []*Mount `json:"mounts"`

	// 容器是否启用 tty
	TTY bool `json:"tty"`

	// 容器的运行命令
	CmdArray []string `json:"CmdArray"`

	// 容器的 Cgroup 配置
	Cgroup *Cgroup `json:"cgroup"`

	// 容器的创建时间
	CreatedTime string `json:"createdTime"`
}
