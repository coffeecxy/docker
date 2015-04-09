// +build linux

package mount

import "github.com/Sirupsen/logrus"

func MakeShared(mountPoint string) error {
	return ensureMountedAs(mountPoint, "shared")
}

func MakeRShared(mountPoint string) error {
	return ensureMountedAs(mountPoint, "rshared")
}

// 将文件系统的挂载点的属性添加private
func MakePrivate(mountPoint string) error {
	return ensureMountedAs(mountPoint, "private")
}

func MakeRPrivate(mountPoint string) error {
	return ensureMountedAs(mountPoint, "rprivate")
}

func MakeSlave(mountPoint string) error {
	return ensureMountedAs(mountPoint, "slave")
}

func MakeRSlave(mountPoint string) error {
	return ensureMountedAs(mountPoint, "rslave")
}

func MakeUnbindable(mountPoint string) error {
	return ensureMountedAs(mountPoint, "unbindable")
}

func MakeRUnbindable(mountPoint string) error {
	return ensureMountedAs(mountPoint, "runbindable")
}

// ensureMountedAs保证mountPoint这个挂载点挂载的时候使用了特定的选项
func ensureMountedAs(mountPoint, options string) error {
	logrus.Infof("[cxy] ensureMount: mountPoint=%s,options=%s", mountPoint, options)
	mounted, err := Mounted(mountPoint)
	if err != nil {
		return err
	}

	// 如果指定的挂载点没有被挂载
	if !mounted {
		if err := Mount(mountPoint, mountPoint, "none", "bind,rw"); err != nil {
			return err
		}
	}
	mounted, err = Mounted(mountPoint)
	if err != nil {
		return err
	}

	return ForceMount("", mountPoint, "none", options)
}
