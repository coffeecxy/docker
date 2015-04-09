package graphdriver

import (
	"path"
	"syscall"
)

// GetFSMagic 得到rootpath这个路径对应的文件系统的magic数
func GetFSMagic(rootpath string) (FsMagic, error) {
	// 必须使用系统调用,使用了statfs系统调用
	var buf syscall.Statfs_t
	if err := syscall.Statfs(path.Dir(rootpath), &buf); err != nil {
		return 0, err
	}
	return FsMagic(buf.Type), nil
}
