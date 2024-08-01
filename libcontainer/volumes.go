package libcontainer

import (
	"fmt"
	"m-docker/libcontainer/config"
	"os"
	"path"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// 将所有指定的 volume 挂载到容器的相应挂载点上
func MountVolumes(conf *config.Config) error {
	mounts := conf.Mounts
	for _, mount := range mounts {
		destInHost := path.Join(conf.Rootfs, mount.Destination)
		if err := mountVolume(mount.Source, destInHost); err != nil {
			return fmt.Errorf("failed to mount volume [%v:%v]: %v", mount.Source, destInHost, err)
		}
		log.Debugf("mount volume [%v:%v] success", mount.Source, destInHost)
	}

	return nil
}

// 使用 bind mount 挂载 volume
func mountVolume(srcInHost string, destInHost string) error {
	// 创建宿主机上的 src 目录
	if err := os.MkdirAll(srcInHost, 0777); err != nil {
		return fmt.Errorf("failed to create src dir: %v", err)
	}
	// 创建宿主机上的 dest 目录
	if err := os.MkdirAll(destInHost, 0777); err != nil {
		return fmt.Errorf("failed to create dest dir: %v", err)
	}

	// 通过 mount 系统调用进行 bind mount
	if err := syscall.Mount(srcInHost, destInHost, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to bind mount: %v", err)
	}

	return nil
}

// 卸载容器的所有 volume
func UmountVolumes(conf *config.Config) {
	mounts := conf.Mounts
	for _, mount := range mounts {
		destInHost := path.Join(conf.Rootfs, mount.Destination)
		syscall.Unmount(destInHost, 0)
	}
}
