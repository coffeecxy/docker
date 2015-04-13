// +build linux

package server

import (
	"fmt"
	"net/http"
	"os"
	"syscall"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/systemd"
)

// 在linux中，才会有fd和unix这两种传输层协议，所以这个文件中有fd,unix,tcp作为传输层协议的三个http server实现

// NewServer sets up the required Server and does protocol specific checking.
//
// 根据使用的proto和addr开启一个HTTP SERVER. 支持现在通用的tcp,unix socket,fd传输层协议
func NewServer(proto, addr string, job *engine.Job) (Server, error) {
	// Basic error and sanity checking
	switch proto {
	case "fd":
		return nil, serveFd(addr, job)
	case "tcp":
		return setupTcpHttp(addr, job)
	case "unix":
		return setupUnixHttp(addr, job)
	default:
		return nil, fmt.Errorf("Invalid protocol format.")
	}
}

// setupUnixHttp使用unix socket作为传输层,在上面运行http协议,新建一个http server
// addr为unix socket的socket文件的path /var/run/docker/xxx.socket
// job中包含了很多参数,这些参数都在起env中
func setupUnixHttp(addr string, job *engine.Job) (*HttpServer, error) {

	// 新建一个路由器,使用了gorilla的库
	r := createRouter(job.Eng, job.GetenvBool("Logging"), job.GetenvBool("EnableCors"), job.Getenv("CorsHeaders"), job.Getenv("Version"))

	// 先删除这个socket文件
	if err := syscall.Unlink(addr); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// 设置这个进程后面创建的文件的mask,0777表示后面创建的文件全部都是可读,可写可执行的
	// 返回的这个mask表示以前的mask,后面defer会恢复这个进程的mask
	mask := syscall.Umask(0777)
	defer syscall.Umask(mask)

	l, err := newListener("unix", addr, job.GetenvBool("BufferRequests"))
	if err != nil {
		return nil, err
	}

	if err := setSocketGroup(addr, job.Getenv("SocketGroup")); err != nil {
		return nil, err
	}

	if err := os.Chmod(addr, 0660); err != nil {
		return nil, err
	}

	// 注意Handler就是这个router,因为起实现了ServeHttp函数
	return &HttpServer{&http.Server{Addr: addr, Handler: r}, l}, nil
}

// serveFd creates an http.Server and sets it up to serve given a socket activated
// argument.
func serveFd(addr string, job *engine.Job) error {
	r := createRouter(job.Eng, job.GetenvBool("Logging"), job.GetenvBool("EnableCors"), job.Getenv("CorsHeaders"), job.Getenv("Version"))

	ls, e := systemd.ListenFD(addr)
	if e != nil {
		return e
	}

	chErrors := make(chan error, len(ls))

	// We don't want to start serving on these sockets until the
	// daemon is initialized and installed. Otherwise required handlers
	// won't be ready.
	<-activationLock

	// Since ListenFD will return one or more sockets we have
	// to create a go func to spawn off multiple serves
	for i := range ls {
		listener := ls[i]
		go func() {
			httpSrv := http.Server{Handler: r}
			chErrors <- httpSrv.Serve(listener)
		}()
	}

	for i := 0; i < len(ls); i++ {
		err := <-chErrors
		if err != nil {
			return err
		}
	}

	return nil
}

// Called through eng.Job("acceptconnections")
// 在docker daemon创建成功之后,会调用这个Job来通知系统其可以接收来自docker client的请求了
func AcceptConnections(job *engine.Job) error {
	// Tell the init daemon we are accepting requests
	go systemd.SdNotify("READY=1")

	// close the lock so the listeners start accepting connections
	if activationLock != nil {
		close(activationLock)
	}

	return nil
}
