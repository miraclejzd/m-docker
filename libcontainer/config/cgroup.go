package config

type Cgroup struct {
	// cgroup 名称
	Name string `json:"name"`

	// cgroup 目录的绝对路径
	Path string `json:"path"`

	*Resources
}

// cgroup 资源限制
type Resources struct {
	// 内存限制
	Memory string

	// CPU 硬限制(hardcapping)的调度周期
	CpuPeriod uint64 `json:"cpuPeriod"`

	// 在 CPU 硬限制的调度周期内，期望使用的 CPU 时间
	CpuQuota uint64 `json:"cpuQuota"`
}
