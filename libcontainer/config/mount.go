package config

// Mount 挂载配置
type Mount struct {
	// 源路径，在宿主机上的绝对路径
	Source string `json:"source"`

	// 目标路径，在容器内的绝对路径
	Destination string `json:"destination"`
}
