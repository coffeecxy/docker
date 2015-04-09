package client

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/docker/docker/pkg/homedir"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/registry"
)

// 表示一个docker client
type DockerCli struct {
	proto      string
	addr       string
	configFile *registry.ConfigFile
	in         io.ReadCloser
	out        io.Writer
	err        io.Writer
	keyFile    string
	tlsConfig  *tls.Config
	scheme     string
	// inFd holds file descriptor of the client's STDIN, if it's a valid file
	inFd uintptr
	// outFd holds file descriptor of the client's STDOUT, if it's a valid file
	outFd uintptr
	// isTerminalIn describes if client's STDIN is a TTY
	isTerminalIn bool
	// isTerminalOut describes if client's STDOUT is a TTY
	isTerminalOut bool
	transport     *http.Transport
}

var funcMap = template.FuncMap{
	"json": func(v interface{}) string {
		a, _ := json.Marshal(v)
		return string(a)
	},
}

// 根据给出的子命令名称,判断这个命令是否存在.
// args可能是一个string,也可能是两个string.
// 如果是一个string,其为命令的名字,如果是两个string,那么为包含了子命令的命令
func (cli *DockerCli) getMethod(args ...string) (func(...string) error, bool) {
	camelArgs := make([]string, len(args))
	for i, s := range args {
		if len(s) == 0 {
			return nil, false
		}
		camelArgs[i] = strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
	}
	//构造出字命令的名字,然后使用反射得到这个函数
	methodName := "Cmd" + strings.Join(camelArgs, "")
	method := reflect.ValueOf(cli).MethodByName(methodName)
	if !method.IsValid() {
		return nil, false
	}
	return method.Interface().(func(...string) error), true
}

// Cmd executes the specified command.
// 传递过来的参数是docker后面的所有参数组成的数组
// 这些命令的名字会被解析出来,然后会运行那个命令
// 当前的args中只包含了命令的部分,docker自己的option部分不会在args中了
func (cli *DockerCli) Cmd(args ...string) error {

	fmt.Println("args of Cmd: ", args)

	//如果给出的参数多于了一个,先看看是不是有命令和子命令的形式
	if len(args) > 1 {
		method, exists := cli.getMethod(args[:2]...)
		if exists {
			return method(args[2:]...)
		}
	}

	//如果有参数,表示给出了一个命令
	if len(args) > 0 {
		method, exists := cli.getMethod(args[0])
		if !exists {
			fmt.Fprintf(cli.err, "docker: '%s' is not a docker command. See 'docker --help'.\n", args[0])
			os.Exit(1)
		}
		fmt.Println("method is:", method)
		// 使用参数调用这个命令
		return method(args[1:]...)
	}

	// 如果给出的命令是有问题的,那么总是输出help
	return cli.CmdHelp()
}

// 根据命令行给出的子命令来得到一个FlagSet
func (cli *DockerCli) Subcmd(name, signature, description string, exitOnError bool) *flag.FlagSet {
	var errorHandling flag.ErrorHandling
	if exitOnError {
		errorHandling = flag.ExitOnError
	} else {
		errorHandling = flag.ContinueOnError
	}
	flags := flag.NewFlagSet(name, errorHandling)

	// 自定义usage函数
	flags.Usage = func() {
		options := ""
		if signature != "" {
			signature = " " + signature
		}
		if flags.FlagCountUndeprecated() > 0 {
			options = " [OPTIONS]"
		}
		fmt.Fprintf(cli.out, "\nUsage: docker %s%s%s\n\n%s\n\n", name, options, signature, description)
		flags.SetOutput(cli.out)
		// 输出ps 的usage
		flags.PrintDefaults()
		os.Exit(0)
	}

	return flags
}

func (cli *DockerCli) LoadConfigFile() (err error) {
	cli.configFile, err = registry.LoadConfig(homedir.Get())
	if err != nil {
		fmt.Fprintf(cli.err, "WARNING: %s\n", err)
	}
	return err
}

func (cli *DockerCli) CheckTtyInput(attachStdin, ttyMode bool) error {
	// In order to attach to a container tty, input stream for the client must
	// be a tty itself: redirecting or piping the client standard input is
	// incompatible with `docker run -t`, `docker exec -t` or `docker attach`.
	if ttyMode && attachStdin && !cli.isTerminalIn {
		return errors.New("cannot enable tty mode on non tty input")
	}
	return nil
}

// 创建一个新的docker client
// in,out,err为这个client的输入/输出/错误流
// keyFile为要使用TLS时的配置
// proto,addr为和docker daemon通信使用的协议和地址
// tlsConfig为使用TLS的配置
func NewDockerCli(in io.ReadCloser, out, err io.Writer, keyFile string, proto, addr string, tlsConfig *tls.Config) *DockerCli {
	var (
		inFd          uintptr
		outFd         uintptr
		isTerminalIn  = false
		isTerminalOut = false
		scheme        = "http"
	)

	// 如果要使用tls,那么就要使用http
	if tlsConfig != nil {
		scheme = "https"
	}
	if in != nil {
		inFd, isTerminalIn = term.GetFdInfo(in)
	}

	if out != nil {
		outFd, isTerminalOut = term.GetFdInfo(out)
	}
	//如果错误流没有设置,设置为输出流
	if err == nil {
		err = out
	}

	// The transport is created here for reuse during the client session
	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// Why 32? See issue 8035
	timeout := 32 * time.Second
	if proto == "unix" {
		// no need in compressing for local communications
		tr.DisableCompression = true
		tr.Dial = func(_, _ string) (net.Conn, error) {
			return net.DialTimeout(proto, addr, timeout)
		}
	} else {
		tr.Proxy = http.ProxyFromEnvironment
		tr.Dial = (&net.Dialer{Timeout: timeout}).Dial
	}

	dockerClient := &DockerCli{
		proto:         proto,
		addr:          addr,
		in:            in,
		out:           out,
		err:           err,
		keyFile:       keyFile,
		inFd:          inFd,
		outFd:         outFd,
		isTerminalIn:  isTerminalIn,
		isTerminalOut: isTerminalOut,
		tlsConfig:     tlsConfig,
		scheme:        scheme,
		transport:     tr,
	}

	fmt.Printf("%+v\n", dockerClient)
	// &{proto:unix addr:/var/run/docker.sock configFile:<nil> in:0xc20802c000 out:0xc20802c008 err:0xc20802c010 keyFile:/home/cxy/.docker/key.json tlsConfig:<nil> scheme:http inFd:0 outFd:1 isTerminalIn:true isTerminalOut:true transport:0xc208084240}

	return dockerClient
}
