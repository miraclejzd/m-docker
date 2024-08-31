package constant

const (
	// cgroupV2 在宿主机上的统一挂载点
	CgroupV2UnifiedMountPoint = "/sys/fs/cgroup"

	// m-docker 的 cgroup 根目录
	CgroupRootPath = "/sys/fs/cgroup/m-docker.slice"

	// m-docker 数据的根目录
	RootPath = "/var/lib/m-docker"

	// m-docker 状态信息的根目录
	StatePath = "/run/m-docker"

	// 容器 Config 文件名
	ConfigName = "config.json"
)
