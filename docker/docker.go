package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/autogen/dockerversion"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/utils"
)

const (
	defaultTrustKeyFile = "key.json"
	defaultCaFile       = "ca.pem"
	defaultKeyFile      = "key.pem"
	defaultCertFile     = "cert.pem"
)

func main() {

	logrus.Infoln("here is a info line")

	if reexec.Init() {
		return
	}

	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := term.StdStreams()

	initLogging(stderr)

	flag.Parse()
	// FIXME: validate daemon flags here

	if *flVersion {
		showVersion()
		return
	}

	if *flLogLevel != "" {
		lvl, err := logrus.ParseLevel(*flLogLevel)
		//		fmt.Println(lvl)
		if err != nil {
			logrus.Fatalf("Unable to parse logging level: %s", *flLogLevel)
		}
		setLogLevel(lvl)
	} else {
		// 默认使用info的log level
		setLogLevel(logrus.InfoLevel)
	}

	// -D, --debug, -l/--log-level=debug processing
	// When/if -D is removed this block can be deleted
	if *flDebug {
		os.Setenv("DEBUG", "1")
		// 如果开启了debug模式,那么log就会变成debug级别
		setLogLevel(logrus.DebugLevel)
	}

	// 解析出host
	if len(flHosts) == 0 {
		defaultHost := os.Getenv("DOCKER_HOST")
		if defaultHost == "" || *flDaemon {
			// If we do not have a host, default to unix socket
			defaultHost = fmt.Sprintf("unix://%s", api.DEFAULTUNIXSOCKET)
		}
		defaultHost, err := api.ValidateHost(defaultHost)
		if err != nil {
			logrus.Fatal(err)
		}

		flHosts = append(flHosts, defaultHost)
		//		fmt.Println(flHosts)
	}

	setDefaultConfFlag(flTrustKey, defaultTrustKeyFile)

	if *flDaemon {
		if *flHelp {
			flag.Usage()
			return
		}
		// 如果要运行一个deamon,从这儿开始
		// 一般来说,daemon都不会自动退出的
		mainDaemon()
		return
	}

	//-----------------------------

	// 下面的代码是client处理的
	if len(flHosts) > 1 {
		logrus.Fatal("Please specify only one -H")
	}
	//将host分成两部分
	protoAddrParts := strings.SplitN(flHosts[0], "://", 2)

	//	fmt.Println(flHosts)
	//	fmt.Println(protoAddrParts) //[unix /var/run/docker.sock]

	var (
		// 表示一个docker client
		cli       *client.DockerCli
		tlsConfig tls.Config
	)
	tlsConfig.InsecureSkipVerify = true

	// Regardless of whether the user sets it to true or false, if they
	// specify --tlsverify at all then we need to turn on tls
	if flag.IsSet("-tlsverify") {
		*flTls = true
	}

	// If we should verify the server, we need to load a trusted ca
	if *flTlsVerify {
		certPool := x509.NewCertPool()
		file, err := ioutil.ReadFile(*flCa)
		if err != nil {
			logrus.Fatalf("Couldn't read ca cert %s: %s", *flCa, err)
		}
		certPool.AppendCertsFromPEM(file)
		tlsConfig.RootCAs = certPool
		tlsConfig.InsecureSkipVerify = false
	}

	// If tls is enabled, try to load and send client certificates
	if *flTls || *flTlsVerify {
		_, errCert := os.Stat(*flCert)
		_, errKey := os.Stat(*flKey)
		if errCert == nil && errKey == nil {
			*flTls = true
			cert, err := tls.LoadX509KeyPair(*flCert, *flKey)
			if err != nil {
				logrus.Fatalf("Couldn't load X509 key pair: %q. Make sure the key is encrypted", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		// Avoid fallback to SSL protocols < TLS1.0
		tlsConfig.MinVersion = tls.VersionTLS10
	}

	if *flTls || *flTlsVerify {
		cli = client.NewDockerCli(stdin, stdout, stderr, *flTrustKey, protoAddrParts[0], protoAddrParts[1], &tlsConfig)
	} else {
		// 创建一个docker client
		cli = client.NewDockerCli(stdin, stdout, stderr, *flTrustKey, protoAddrParts[0], protoAddrParts[1], nil)
	}

	// docker client开始处理命令
	if err := cli.Cmd(flag.Args()...); err != nil {
		if sterr, ok := err.(*utils.StatusError); ok {
			if sterr.Status != "" {
				logrus.Println(sterr.Status)
			}
			os.Exit(sterr.StatusCode)
		}
		logrus.Fatal(err)
	}
}

func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", dockerversion.VERSION, dockerversion.GITCOMMIT)
}
