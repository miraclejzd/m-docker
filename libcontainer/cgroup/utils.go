package cgroup

import (
	"m-docker/libcontainer/constant"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

var (
	isUnifiedOnce sync.Once // sync.Once 用于确保某种操作只进行一次
	isUnified     bool
)

// IsCgroup2UnifiedMode 检查 cgroup v2 是否启用
func IsCgroup2UnifiedMode() bool {
	// 使用 sync.Once 来确保检查 cgroup v2 的操作只进行一次
	// 目的只是为了提高性能，避免重复检查
	isUnifiedOnce.Do(func() {
		var st unix.Statfs_t
		err := unix.Statfs(constant.CgroupV2UnifiedMountPoint, &st)

		// 如果 unifiedMountPoint 不存在，则 cgroup v2 肯定未启用
		if err != nil && os.IsNotExist(err) {
			isUnified = false
		} else { // 若 unifiedMountPoint 存在，则还需要根据目录类型判断 cgroup v2 是否启用
			isUnified = (st.Type == unix.CGROUP2_SUPER_MAGIC)
		}
	})
	return isUnified
}
