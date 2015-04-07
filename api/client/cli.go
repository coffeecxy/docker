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

// DockerCli表示docker的client部分
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

// 根据给出的子命令名称,判断这个字命令是否存在.
// 存在的话,返回其对应的函数.
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
// 这些子命令的名字会被解析出来,然后会运行那个命令
func (cli *DockerCli) Cmd(args ...string) error {
	//如果给出的参数多于了一个,也就是包含了子命令以及其参数
	if len(args) > 1 {
		method, exists := cli.getMethod(args[:2]...)
		if exists {
			return method(args[2:]...)
		}
	}

	if len(args) > 0 {
		method, exists := cli.getMethod(args[0])
		if !exists {
			fmt.Fprintf(cli.err, "docker: '%s' is not a docker command. See 'docker --help'.\n", args[0])
			os.Exit(1)
		}
		fmt.Println(method)
		// 使用
		return method(args[1:]...)
	}

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

func NewDockerCli(in io.ReadCloser, out, err io.Writer, keyFile string, proto, addr string, tlsConfig *tls.Config) *DockerCli {
	var (
		inFd          uintptr
		outFd         uintptr
		isTerminalIn  = false
		isTerminalOut = false
		scheme        = "http"
	)

	if tlsConfig != nil {
		scheme = "https"
	}
	if in != nil {
		inFd, isTerminalIn = term.GetFdInfo(in)
	}

	if out != nil {
		outFd, isTerminalOut = term.GetFdInfo(out)
	}

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

	return &DockerCli{
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
}
