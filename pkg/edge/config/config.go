package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/config"
	"github.com/liaisonio/liaison/pkg/utils"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v2"
)

// ErrShowFingerprint 表示需要显示指纹并退出的错误
var ErrShowFingerprint = errors.New("show fingerprint and exit")

var (
	Conf      *Configuration
	RotateLog *lumberjack.Logger

	h               bool
	showFingerprint bool
	file            string
	defaultFile     string = "./liaison-edge.yaml"
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

type Auth struct {
	AccessKey string `yaml:"access_key,omitempty" json:"access_key"`
	SecretKey string `yaml:"secret_key,omitempty" json:"secret_key"`
}

type Manager struct {
	Dial config.Dial `yaml:"dial,omitempty" json:"dial"`
	Auth Auth        `yaml:"auth,omitempty" json:"auth"`
}

type Log struct {
	Level    string `yaml:"level"`
	File     string `yaml:"file"`
	MaxSize  int    `yaml:"maxsize"`
	MaxRolls int    `yaml:"maxrolls"`
}

type Configuration struct {
	Daemon  Daemon  `yaml:"daemon,omitempty" json:"daemon"`
	Manager Manager `yaml:"manager,omitempty" json:"manager"`
	Log     Log     `yaml:"log,omitempty" json:"log"`
}

func initCmd() error {
	flag.StringVar(&file, "c", defaultFile, "configuration file")
	flag.BoolVar(&h, "h", false, "help")
	flag.BoolVar(&showFingerprint, "fingerprint", false, "show device fingerprint and exit")
	flag.Parse()
	if h {
		flag.Usage()
		return fmt.Errorf("invalid usage for command line")
	}
	if showFingerprint {
		showFingerprintDetails()
		return ErrShowFingerprint
	}
	return nil
}

// ShouldShowFingerprint 返回是否需要显示指纹
func ShouldShowFingerprint() bool {
	return showFingerprint
}

// showFingerprintDetails 显示详细的指纹信息
func showFingerprintDetails() {
	// 使用工具函数获取指纹和组成信息
	fingerprint, components, err := utils.GetFingerprint()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting fingerprint: %v\n", err)
		os.Exit(1)
	}

	if components == nil {
		fmt.Fprintf(os.Stderr, "Error: fingerprint components is nil\n")
		os.Exit(1)
	}

	// 打印 Debug 日志
	log.Debugf("showFingerprintDetails fingerprint: %s, MACs: %v, CPU: %s, DiskIDs: %v, Raw: %s",
		fingerprint, components.MACs, components.CPUInfo, components.DiskIDs, components.Raw)

	// 显示详细信息
	fmt.Println("\n=== Fingerprint Details ===")
	fmt.Printf("MAC Addresses (used):\n")
	for i, mac := range components.MACs {
		fmt.Printf("  [%d] %s\n", i+1, mac)
	}
	fmt.Printf("\nCPU Info: %s\n", components.CPUInfo)
	fmt.Printf("Disk Serial Numbers:\n")
	for i, diskID := range components.DiskIDs {
		fmt.Printf("  [%d] %s\n", i+1, diskID)
	}
	fmt.Printf("\nRaw String: %s\n", components.Raw)
	fmt.Printf("Fingerprint: %s\n", fingerprint)
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

	return nil
}

func initConf() error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	Conf = &Configuration{}
	err = yaml.Unmarshal([]byte(data), Conf)
	return err
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
