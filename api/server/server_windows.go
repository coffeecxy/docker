// +build windows

package server

import (
	"fmt"

	"github.com/docker/docker/engine"
)

// 在windows中,没有unix和fd这两个协议,只能使用tcp

// NewServer sets up the required Server and does protocol specific checking.
func NewServer(proto, addr string, job *engine.Job) (Server, error) {
	// Basic error and sanity checking
	switch proto {
	case "tcp":
		return setupTcpHttp(addr, job)
	default:
		return nil, errors.New("Invalid protocol format. Windows only supports tcp.")
	}
}

// Called through eng.Job("acceptconnections")
func AcceptConnections(job *engine.Job) engine.Status {

	// close the lock so the listeners start accepting connections
	if activationLock != nil {
		close(activationLock)
	}

	return engine.StatusOK
}
