package sysinfo

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libcontainer/cgroups"
)

// SysInfo stores information about which features a kernel supports.
type SysInfo struct {
	MemoryLimit            bool
	SwapLimit              bool
	IPv4ForwardingDisabled bool
	AppArmor               bool
}

// New returns a new SysInfo, using the filesystem to detect which features the kernel supports.
// 检测kernel支持的特性,如果quiet为false,那么如果内核不支持某些特性,会有warning输出
func New(quiet bool) *SysInfo {
	sysInfo := &SysInfo{}
	// /sys/fs/cgroup,在我的系统中,cgroup挂载在这个路径上
	if cgroupMemoryMountpoint, err := cgroups.FindCgroupMountpoint("memory"); err != nil {
		if !quiet {
			logrus.Warnf("%s", err)
		}
	} else {
		// /sys/fs/cgroup/memory目录中包含了关于cgroup memory的配置
		_, err1 := ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.limit_in_bytes"))
		_, err2 := ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.soft_limit_in_bytes"))
		sysInfo.MemoryLimit = err1 == nil && err2 == nil
		if !sysInfo.MemoryLimit && !quiet {
			logrus.Warnf("Your kernel does not support cgroup memory limit.")
		}

		_, err = ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.memsw.limit_in_bytes"))
		sysInfo.SwapLimit = err == nil
		// 我的系统中是有这个文件的,所以会输出下面的warning
		if !sysInfo.SwapLimit && !quiet {
			logrus.Warnf("Your kernel does not support cgroup swap limit.")
		}
	}

	// Check if AppArmor is supported.
	if _, err := os.Stat("/sys/kernel/security/apparmor"); os.IsNotExist(err) {
		sysInfo.AppArmor = false
	} else {
		sysInfo.AppArmor = true
	}
	return sysInfo
}
