package config

import (
	"flag"
	"os"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/config"
	"github.com/liaisonio/liaison/pkg/lerrors"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v2"
)

var (
	Conf      *Configuration
	RotateLog *lumberjack.Logger

	h           bool
	file        string
	defaultFile string = "./liaison.yaml"
)

// daemon related
type RLimit struct {
	Enable  bool `yaml:"enable" json:"enable"`
	NumFile int  `yaml:"nofile" json:"nofile"`
}

type PProf struct {
	Enable         bool   `yaml:"enable" json:"enable"`
	Addr           string `yaml:"addr" json:"addr"`
	CPUProfileRate int    `yaml:"cpu_profile_rate" json:"cpu_profile_rate"`
}

type Daemon struct {
	RLimit RLimit `yaml:"rlimit,omitempty" json:"rlimit"`
	PProf  PProf  `yaml:"pprof,omitempty" json:"pprof"`
}

type Agent struct {
	Enabled        bool          `yaml:"enabled,omitempty" json:"enabled"`
	BaseURL        string        `yaml:"base_url,omitempty" json:"base_url"`
	Model          string        `yaml:"model,omitempty" json:"model"`
	APIKeyEnv      string        `yaml:"api_key_env,omitempty" json:"api_key_env"`
	RequestTimeout time.Duration `yaml:"request_timeout,omitempty" json:"request_timeout"`
	MaxModelSteps  int           `yaml:"max_model_steps,omitempty" json:"max_model_steps"`
	ApprovalExpiry time.Duration `yaml:"approval_expiry,omitempty" json:"approval_expiry"`
}

type Manager struct {
	Listen           config.Listen `yaml:"listen,omitempty" json:"listen"`
	DB               string        `yaml:"db,omitempty" json:"db"`
	ServerURL        string        `yaml:"server_url,omitempty" json:"server_url"`                 // 服务器地址，用于生成安装命令
	PackagesDir      string        `yaml:"packages_dir,omitempty" json:"packages_dir"`             // 安装包目录，默认 /opt/liaison/packages
	WebDir           string        `yaml:"web_dir,omitempty" json:"web_dir"`                       // 前端文件目录，如果为空则不提供前端服务
	FrontierEdgePort int           `yaml:"frontier_edge_port,omitempty" json:"frontier_edge_port"` // Edge 和 Frontier 之间的通信端口
	JWTSecret        string        `yaml:"jwt_secret,omitempty" json:"jwt_secret"`                 // JWT 密钥（必需，至少32字符）
	CredentialSecret string        `yaml:"credential_secret,omitempty" json:"credential_secret"`   // WebSSH 凭据加密密钥；为空时回退使用 JWTSecret
	SSHHostKeyFile   string        `yaml:"ssh_host_key_file,omitempty" json:"ssh_host_key_file"`   // 原生 SSH Gateway 的持久化 Host Key
	SSHIdleTimeout   time.Duration `yaml:"ssh_idle_timeout,omitempty" json:"ssh_idle_timeout"`     // 原生 SSH 会话空闲超时
	SSHMaxDuration   time.Duration `yaml:"ssh_max_duration,omitempty" json:"ssh_max_duration"`     // 原生 SSH 会话最长持续时间
	GuacdAddr        string        `yaml:"guacd_addr,omitempty" json:"guacd_addr"`                 // guacd 地址，用于 WebDesktop
	GuacdBridgeAddr  string        `yaml:"guacd_bridge_addr,omitempty" json:"guacd_bridge_addr"`   // manager 本地临时桥接监听地址
	GuacdBridgeHost  string        `yaml:"guacd_bridge_host,omitempty" json:"guacd_bridge_host"`   // guacd 回连 manager 临时桥接端口时使用的主机名
	Agent            Agent         `yaml:"agent,omitempty" json:"agent"`
}

type Frontier struct {
	Dial            config.Dial `yaml:"dial,omitempty" json:"dial"`
	ControlPlaneURL string      `yaml:"controlplane_url,omitempty" json:"controlplane_url"`
}

type Log struct {
	Level    string `yaml:"level"`
	File     string `yaml:"file"`
	MaxSize  int    `yaml:"maxsize"`
	MaxRolls int    `yaml:"maxrolls"`
}

type Configuration struct {
	Daemon   Daemon   `yaml:"daemon,omitempty" json:"daemon"`
	Manager  Manager  `yaml:"manager,omitempty" json:"manager"`
	Frontier Frontier `yaml:"frontier,omitempty" json:"frontier"`

	Log Log `yaml:"log"`
}

func Init() error {
	time.LoadLocation("Asia/Shanghai")

	err := initCmd()
	if err != nil {
		return err
	}

	err = initConf()
	if err != nil {
		return err
	}

	err = initLog()
	if err != nil {
		return err
	}
	return err
}

func initCmd() error {
	flag.StringVar(&file, "c", defaultFile, "configuration file")
	flag.BoolVar(&h, "h", false, "help")
	flag.Parse()
	if h {
		flag.Usage()
		return lerrors.ErrInvalidUsage
	}
	return nil
}

func initConf() error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	Conf = &Configuration{}
	err = yaml.Unmarshal([]byte(data), Conf)
	if err != nil {
		return err
	}
	if Conf.Manager.FrontierEdgePort == 0 {
		Conf.Manager.FrontierEdgePort = 30012
	}
	if Conf.Manager.GuacdAddr == "" {
		Conf.Manager.GuacdAddr = "127.0.0.1:4822"
	}
	if Conf.Manager.GuacdBridgeAddr == "" {
		Conf.Manager.GuacdBridgeAddr = "127.0.0.1:0"
	}
	if Conf.Manager.SSHHostKeyFile == "" {
		Conf.Manager.SSHHostKeyFile = "/opt/liaison/data/ssh_host_ed25519_key"
	}
	if Conf.Manager.SSHIdleTimeout == 0 {
		Conf.Manager.SSHIdleTimeout = 30 * time.Minute
	}
	if Conf.Manager.SSHMaxDuration == 0 {
		Conf.Manager.SSHMaxDuration = 8 * time.Hour
	}
	if Conf.Manager.Agent.APIKeyEnv == "" {
		Conf.Manager.Agent.APIKeyEnv = "OPENAI_API_KEY"
	}
	if Conf.Manager.Agent.RequestTimeout == 0 {
		Conf.Manager.Agent.RequestTimeout = 2 * time.Minute
	}
	if Conf.Manager.Agent.MaxModelSteps == 0 {
		Conf.Manager.Agent.MaxModelSteps = 12
	}
	if Conf.Manager.Agent.ApprovalExpiry == 0 {
		Conf.Manager.Agent.ApprovalExpiry = 15 * time.Minute
	}
	return nil
}

func initLog() error {
	level, err := log.ParseLevel(Conf.Log.Level)
	if err != nil {
		return err
	}
	log.SetLevel(level)
	RotateLog = &lumberjack.Logger{
		Filename:   Conf.Log.File,
		MaxSize:    Conf.Log.MaxSize,
		MaxBackups: Conf.Log.MaxRolls,
	}
	log.SetOutput(RotateLog)
	return nil
}
