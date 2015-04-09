package daemon

import (
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/networkdriver"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/ulimit"
	"github.com/docker/docker/runconfig"
)

const (
	defaultNetworkMtu    = 1500
	disableNetworkBridge = "none"
)

// Config define the configuration of a docker daemon
// These are the configuration settings that you pass
// to the docker daemon when you launch it with say: `docker -d -e lxc`
// FIXME: separate runtime configuration from http api configuration
// 包含一个docker deamon运行的配置信息,这个在新建一个docker daemon之前会被成功的
// 根据命令行的选项配置好
type Config struct {
	Pidfile                     string   // 所属进程的pid文件
	Root                        string   // 运行的时候的root路径,默认是/var/lib/docker
	AutoRestart                 bool     //现在默认就是自动重启了
	Dns                         []string // docker使用的dns的ip地址
	DnsSearch                   []string // dns域名
	EnableIPv6                  bool
	EnableIptables              bool
	EnableIpForward             bool
	EnableIpMasq                bool   // ip伪装技术
	DefaultIp                   net.IP //绑定端口时使用的默认ip
	BridgeIface                 string
	BridgeIP                    string
	FixedCIDR                   string
	FixedCIDRv6                 string
	InterContainerCommunication bool     //是否允许容器间进行通信
	GraphDriver                 string   //docker运行的时候使用的存储驱动 默认值为空
	GraphOptions                []string //存储驱动选项
	ExecDriver                  string   //exec驱动
	Mtu                         int      //容器网络的MTU
	SocketGroup                 string
	EnableCors                  bool
	CorsHeaders                 string
	DisableNetwork              bool
	EnableSelinuxSupport        bool //启用selinux
	Context                     map[string][]string
	TrustKeyPath                string
	Labels                      []string
	Ulimits                     map[string]*ulimit.Ulimit
	LogConfig                   runconfig.LogConfig
}

// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
// Subsequent calls to `flag.Parse` will populate config with values parsed
// from the command-line.
func (config *Config) InstallFlags() {
	flag.StringVar(&config.Pidfile, []string{"p", "-pidfile"}, "/var/run/docker.pid", "Path to use for daemon PID file")
	flag.StringVar(&config.Root, []string{"g", "-graph"}, "/var/lib/docker", "Root of the Docker runtime")
	flag.BoolVar(&config.AutoRestart, []string{"#r", "#-restart"}, true, "--restart on the daemon has been deprecated in favor of --restart policies on docker run")
	flag.BoolVar(&config.EnableIptables, []string{"#iptables", "-iptables"}, true, "Enable addition of iptables rules")
	flag.BoolVar(&config.EnableIpForward, []string{"#ip-forward", "-ip-forward"}, true, "Enable net.ipv4.ip_forward")
	flag.BoolVar(&config.EnableIpMasq, []string{"-ip-masq"}, true, "Enable IP masquerading")
	flag.BoolVar(&config.EnableIPv6, []string{"-ipv6"}, false, "Enable IPv6 networking")
	flag.StringVar(&config.BridgeIP, []string{"#bip", "-bip"}, "", "Specify network bridge IP")
	flag.StringVar(&config.BridgeIface, []string{"b", "-bridge"}, "", "Attach containers to a network bridge")
	flag.StringVar(&config.FixedCIDR, []string{"-fixed-cidr"}, "", "IPv4 subnet for fixed IPs")
	flag.StringVar(&config.FixedCIDRv6, []string{"-fixed-cidr-v6"}, "", "IPv6 subnet for fixed IPs")
	flag.BoolVar(&config.InterContainerCommunication, []string{"#icc", "-icc"}, true, "Enable inter-container communication")
	flag.StringVar(&config.GraphDriver, []string{"s", "-storage-driver"}, "", "Storage driver to use")
	flag.StringVar(&config.ExecDriver, []string{"e", "-exec-driver"}, "native", "Exec driver to use")
	flag.BoolVar(&config.EnableSelinuxSupport, []string{"-selinux-enabled"}, false, "Enable selinux support")
	flag.IntVar(&config.Mtu, []string{"#mtu", "-mtu"}, 0, "Set the containers network MTU")
	flag.StringVar(&config.SocketGroup, []string{"G", "-group"}, "docker", "Group for the unix socket")
	flag.BoolVar(&config.EnableCors, []string{"#api-enable-cors", "#-api-enable-cors"}, false, "Enable CORS headers in the remote API, this is deprecated by --api-cors-header")
	flag.StringVar(&config.CorsHeaders, []string{"-api-cors-header"}, "", "Set CORS headers in the remote API")
	opts.IPVar(&config.DefaultIp, []string{"#ip", "-ip"}, "0.0.0.0", "Default IP when binding container ports")
	opts.ListVar(&config.GraphOptions, []string{"-storage-opt"}, "Set storage driver options")
	// FIXME: why the inconsistency between "hosts" and "sockets"?
	opts.IPListVar(&config.Dns, []string{"#dns", "-dns"}, "DNS server to use")
	opts.DnsSearchListVar(&config.DnsSearch, []string{"-dns-search"}, "DNS search domains to use")
	opts.LabelListVar(&config.Labels, []string{"-label"}, "Set key=value labels to the daemon")
	config.Ulimits = make(map[string]*ulimit.Ulimit)
	opts.UlimitMapVar(config.Ulimits, []string{"-default-ulimit"}, "Set default ulimits for containers")
	flag.StringVar(&config.LogConfig.Type, []string{"-log-driver"}, "json-file", "Containers logging driver")
}

func getDefaultNetworkMtu() int {
	if iface, err := networkdriver.GetDefaultRouteIface(); err == nil {
		logrus.Infoln("[cxy] deamonConfig: use interface mtu")
		return iface.MTU
	}
	logrus.Infoln("[cxy] deamonConfig: use default mtu")
	return defaultNetworkMtu
}
