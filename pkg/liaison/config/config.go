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

type Manager struct {
	Listen           config.Listen `yaml:"listen,omitempty" json:"listen"`
	DB               string        `yaml:"db,omitempty" json:"db"`
	ServerURL        string        `yaml:"server_url,omitempty" json:"server_url"`                 // 服务器地址，用于生成安装命令
	PackagesDir      string        `yaml:"packages_dir,omitempty" json:"packages_dir"`             // 安装包目录，默认 /opt/liaison/packages
	WebDir           string        `yaml:"web_dir,omitempty" json:"web_dir"`                       // 前端文件目录，如果为空则不提供前端服务
	FrontierEdgePort int           `yaml:"frontier_edge_port,omitempty" json:"frontier_edge_port"` // Edge 和 Frontier 之间的通信端口
	JWTSecret        string        `yaml:"jwt_secret,omitempty" json:"jwt_secret"`                // JWT 密钥（必需，至少32字符）
}

type Frontier struct {
	Dial config.Dial `yaml:"dial,omitempty" json:"dial"`
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
