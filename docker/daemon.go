// +build daemon

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builtins"
	"github.com/docker/docker/daemon"
	_ "github.com/docker/docker/daemon/execdriver/lxc"
	_ "github.com/docker/docker/daemon/execdriver/native"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/homedir"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/timeutils"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

// 如果使用了-d模式,那么会开启daemon

const CanDaemon = true

// 初始化daemon的配置
var (
	daemonCfg   = &daemon.Config{}
	registryCfg = &registry.Options{}
)

func init() {
	daemonCfg.InstallFlags()
	registryCfg.InstallFlags()
}

// 在以前的版本中,docker key存放的地方和现在版本存放的地方是不同的了,
// 这个函数完成将key进行迁移
func migrateKey() (err error) {
	// Migrate trust key if exists at ~/.docker/key.json and owned by current user
	oldPath := filepath.Join(homedir.Get(), ".docker", defaultTrustKeyFile)
	newPath := filepath.Join(getDaemonConfDir(), defaultTrustKeyFile)
	if _, statErr := os.Stat(newPath); os.IsNotExist(statErr) && utils.IsFileOwner(oldPath) {
		defer func() {
			// Ensure old path is removed if no error occurred
			if err == nil {
				err = os.Remove(oldPath)
			} else {
				logrus.Warnf("Key migration failed, key file not removed at %s", oldPath)
			}
		}()

		if err := os.MkdirAll(getDaemonConfDir(), os.FileMode(0644)); err != nil {
			return fmt.Errorf("Unable to create daemon configuration directory: %s", err)
		}

		newFile, err := os.OpenFile(newPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("error creating key file %q: %s", newPath, err)
		}
		defer newFile.Close()

		oldFile, err := os.Open(oldPath)
		if err != nil {
			return fmt.Errorf("error opening key file %q: %s", oldPath, err)
		}
		defer oldFile.Close()

		if _, err := io.Copy(newFile, oldFile); err != nil {
			return fmt.Errorf("error copying key: %s", err)
		}

		logrus.Infof("Migrated key from %s to %s", oldPath, newPath)
	}

	return nil
}

func mainDaemon() {

	// 如果解析了docker的选项之后还剩下了其他的参数,那么其为命令
	// 那么这个docker应该是一个client,所以就要退出
	if flag.NArg() != 0 {
		flag.Usage()
		return
	}

	// 设置log的时间格式
	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: timeutils.RFC3339NanoFixed})

	// 新建一个Engine
	eng := engine.New()

	// eng.Shutdown为docker要退出的时候要使用的clearup函数
	// 因为docker daemon自己是不会退出的,所以都是由其他的信号过来导致的退出,这里就是
	// 处理退出之前要做的事情
	signal.Trap(eng.Shutdown)

	if err := migrateKey(); err != nil {
		logrus.Fatal(err)
	}
	daemonCfg.TrustKeyPath = *flTrustKey

	// Load builtins
	if err := builtins.Register(eng); err != nil {
		logrus.Fatal(err)
	}

	// load registry service
	if err := registry.NewService(registryCfg).Install(eng); err != nil {
		logrus.Fatal(err)
	}

	// 到这儿,eng中已经注册了一些handler了可以使用了

	// load the daemon in the background so we can immediately start
	// the http api so that connections don't fail while the daemon
	// is booting
	daemonInitWait := make(chan error)
	// 在后台启动一个docker daemon,这样当前routine还可以处理serveapi的初始化工作
	go func() {
		// 使用init中初始化的deamonCfg和上面配置好的eng来初始化一个daemon,这里面的逻辑很多
		d, err := daemon.NewDaemon(daemonCfg, eng)
		if err != nil {
			daemonInitWait <- err
			return
		}

		// 在调试的时候,会发现这个log并不是马上就出现了,因为其是在另外的一个routine
		// 中运行的,所以输出的时间是不定的
		logrus.Infof("docker daemon: %s %s; execdriver: %s; graphdriver: %s",
			dockerversion.VERSION,
			dockerversion.GITCOMMIT,
			d.ExecutionDriver().Name(),
			d.GraphDriver().String(),
		)

		if err := d.Install(eng); err != nil {
			daemonInitWait <- err
			return
		}

		b := &builder.BuilderJob{eng, d}
		b.Install()

		// after the daemon is done setting up we can tell the api to start
		// accepting connections
		// 成功启动了docker daemon之后,运行acceptconnections这个job,让系统知道
		// 这个daemon可以接收进来的连接了
		if err := eng.Job("acceptconnections").Run(); err != nil {
			daemonInitWait <- err
			return
		}
		daemonInitWait <- nil
	}()

	// Serve api
	// 在执行serveapi这个job之前,设置其环境变量,这个job的参数为daemon要支持的proto数组
	// 在这个routine中,开启server api
	// [unix:///var/run/docker.sock]
	job := eng.Job("serveapi", flHosts...)
	logrus.Infof("[cxy] serveapi: hosts: %v", flHosts)

	job.SetenvBool("Logging", true)
	job.SetenvBool("EnableCors", daemonCfg.EnableCors)
	job.Setenv("CorsHeaders", daemonCfg.CorsHeaders)
	job.Setenv("Version", dockerversion.VERSION)
	job.Setenv("SocketGroup", daemonCfg.SocketGroup)

	job.SetenvBool("Tls", *flTls)
	job.SetenvBool("TlsVerify", *flTlsVerify)
	job.Setenv("TlsCa", *flCa)
	job.Setenv("TlsCert", *flCert)
	job.Setenv("TlsKey", *flKey)
	job.SetenvBool("BufferRequests", true)

	// The serve API job never exits unless an error occurs
	// We need to start it as a goroutine and wait on it so
	// daemon doesn't exit
	// serveapi对应的handler运行的时候是不会停止的,所以必须把它放到另外一个routine中去
	serveAPIWait := make(chan error)
	go func() {
		// 运行这个serveapi对应的handler
		if err := job.Run(); err != nil {
			logrus.Errorf("ServeAPI error: %v", err)
			serveAPIWait <- err
			return
		}
		// 一般的,serveapi这个job不会对应的routine会被阻塞,这儿不会被执行
		serveAPIWait <- nil
	}()

	// Wait for the daemon startup goroutine to finish
	// This makes sure we can actually cleanly shutdown the daemon
	logrus.Debug("waiting for daemon to initialize")
	// 如果从daemonInitWait中读出了值,说明用于初始化daemon的routine完成了
	errDaemon := <-daemonInitWait
	if errDaemon != nil {
		eng.Shutdown()
		outStr := fmt.Sprintf("Shutting down daemon due to errors: %v", errDaemon)
		if strings.Contains(errDaemon.Error(), "engine is shutdown") {
			// if the error is "engine is shutdown", we've already reported (or
			// will report below in API server errors) the error
			outStr = "Shutting down daemon due to reported errors"
		}
		// we must "fatal" exit here as the API server may be happy to
		// continue listening forever if the error had no impact to API
		logrus.Fatal(outStr)
	} else {
		logrus.Info("Daemon has completed initialization")
	}

	// Daemon is fully initialized and handling API traffic
	// Wait for serve API job to complete
	// 如果serveAPIWait中可以读出值了,说明被外部的kill中断了
	errAPI := <-serveAPIWait
	// If we have an error here it is unique to API (as daemonErr would have
	// exited the daemon process above)
	logrus.Infof("[cxy] deamon: serveapi end")
	eng.Shutdown()
	if errAPI != nil {
		logrus.Fatalf("Shutting down due to ServeAPI error: %v", errAPI)
	}
}
