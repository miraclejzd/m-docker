package libcontainer

import (
	"fmt"
	"m-docker/libcontainer/config"
	"os"
	"os/exec"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
)

// m-docker 数据的根目录
var rootPath = "/var/lib/m-docker"

// 创建容器的 rootfs 目录
func CreateRootfs(conf *config.Config) error {
	imagePath := path.Join(rootPath, "images", "ubuntu.tar")
	imageLayerPath := path.Join(rootPath, "layers", "ubuntu")

	// 首先解压镜像
	if err := unzipImageLayer(imagePath, imageLayerPath); err != nil {
		return fmt.Errorf("fail to unzip image layer: %v", err)
	}

	// 之后准备 overlay 所需要的目录
	if err := prepareOverlayDir(conf.RwLayer, conf.Rootfs); err != nil {
		return fmt.Errorf("fail to prepare overlay dir:  %v", err)
	}

	// 最后使用 overlay 将镜像层读写层叠加到 rootfs 上
	if err := mountRootfs([]string{imageLayerPath}, conf.RwLayer, conf.Rootfs); err != nil {
		_ = os.RemoveAll(conf.RwLayer)
		return fmt.Errorf("fail to mount rootfs: %v", err)
	}

	return nil
}

// 将镜像解压到指定目录下
func unzipImageLayer(imagePath string, dest string) error {
	exist, err := pathExists(dest)
	if err != nil {
		return fmt.Errorf("unable to judge whether dir %s exists. %v", dest, err)
	}

	// 镜像不存在
	if !exist {
		// 新建目录
		if err = os.Mkdir(dest, 0755); err != nil {
			return fmt.Errorf("fail to create dir %s:  %v", dest, err)
		}
		// tar -xvf 命令解压镜像
		if err = exec.Command("tar", "-xvf", imagePath, "-C", dest).Run(); err != nil {
			return fmt.Errorf("fail to unzip image %v: %v", imagePath, err)
		}
	}
	// 若镜像已经存在，则无需解压，直接返回
	return nil
}

// 创建 overlay 所需要的目录
func prepareOverlayDir(rwLayerPath string, rootfsPath string) error {
	// 要创建的目录有 4 个
	// rwLayerPath，upper 和 work 的父目录
	// rwLayerPath/fs，作为 upper 目录
	// rwLayerPath/work，作为 work 目录
	// rootfsPath, 联合挂载点
	dirs := []string{
		rwLayerPath,
		path.Join(rwLayerPath, "fs"),
		path.Join(rwLayerPath, "work"),
		rootfsPath,
	}

	for _, dir := range dirs {
		if err := os.Mkdir(dir, 0755); err != nil {
			_ = os.RemoveAll(rwLayerPath)
			return fmt.Errorf("fail to create dir %s: %v", dir, err)
		}
	}

	return nil
}

// 使用 overlay 进行联合挂载
func mountRootfs(lowerDir []string, rwLayerDir string, rootfs string) error {
	// 拼接参数
	overlayArgs := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
		strings.Join(lowerDir, ":"),
		path.Join(rwLayerDir, "fs"),
		path.Join(rwLayerDir, "work"))

	// 完整命令：mount -t overlay m-docker-overlay lowerdir=xxx,upperdir=xxx,workdir=xxx xxx
	cmd := exec.Command("mount", "-t", "overlay", "m-docker-overlay", "-o", overlayArgs, rootfs)
	log.Debugf("Mount overlay command: %v", cmd.String())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fail to overlay mount: %v", err)
	}

	return nil
}

// 当容器退出后，删除 rootfs 相关的目录
func DeleteRootfs(conf *config.Config) {
	umountRootfs(conf.Rootfs)
	deleteOverlayDir(conf.RwLayer, conf.Rootfs)
}

// 解除 overlay 挂载
func umountRootfs(mountPoint string) {
	_ = exec.Command("umount", mountPoint).Run()
}

// 删除 overlay 所准备的目录
func deleteOverlayDir(rwLayerPath string, rootfsPath string) {
	_ = os.RemoveAll(rootfsPath)
	_ = os.RemoveAll(rwLayerPath)
}

// 判断路径目标是否存在
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
