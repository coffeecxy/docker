// +build linux

package daemon

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/libcontainer/selinux"
)

func selinuxSetDisabled() {
	logrus.Debugf("[cxy] selinx disabled")
	selinux.SetDisabled()
}

func selinuxFreeLxcContexts(label string) {
	selinux.FreeLxcContexts(label)
}

func selinuxEnabled() bool {
	return selinux.SelinuxEnabled()
}
