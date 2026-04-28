package factory

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultConfigPath = "./config/dsmfcfg.yaml"

type Config struct {
	Info          Info          `yaml:"info"`
	Configuration Configuration `yaml:"configuration"`
	Logger        Logger        `yaml:"logger"`
}

type Info struct {
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

type Configuration struct {
	DsmfName         string         `yaml:"dsmfName"`
	Sbi              Sbi            `yaml:"sbi"`
	Grpc             Grpc           `yaml:"grpc"`
	DpfEndpoints     []EndpointRef  `yaml:"dpfEndpoints"`
	DsfEndpoints     []EndpointRef  `yaml:"dsfEndpoints"`
	DefaultProtocols ProtocolConfig `yaml:"defaultProtocols"`
	Task             TaskConfig     `yaml:"task"`
}

type Sbi struct {
	Scheme       string `yaml:"scheme"`
	BindingIPv4  string `yaml:"bindingIPv4"`
	RegisterIPv4 string `yaml:"registerIPv4"`
	Port         int    `yaml:"port"`
}

type Grpc struct {
	BindingIPv4 string `yaml:"bindingIPv4"`
	Port        int    `yaml:"port"`
}

type EndpointRef struct {
	ID           string `yaml:"id"`
	Address      string `yaml:"address"`
	HTTP3Address string `yaml:"http3Address"`
	QUICAddress  string `yaml:"quicAddress"`
}

type ProtocolConfig struct {
	Transport string `yaml:"transport"`
	Payload   string `yaml:"payload"`
}

type TaskConfig struct {
	TimeoutSeconds int           `yaml:"timeoutSeconds"`
	Storage        StorageConfig `yaml:"storage"`
}

type StorageConfig struct {
	BackendType       string `yaml:"backendType"`
	ObjectPrefix      string `yaml:"objectPrefix"`
	RetentionDays     uint32 `yaml:"retentionDays"`
	OverwriteIfExists bool   `yaml:"overwriteIfExists"`
}

type Logger struct {
	Enable       bool   `yaml:"enable"`
	Level        string `yaml:"level"`
	ReportCaller bool   `yaml:"reportCaller"`
}

func ReadConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, err
	}
	cfg.setDefaults()
	return cfg, nil
}

func (c *Config) setDefaults() {
	if c.Configuration.DsmfName == "" {
		c.Configuration.DsmfName = "DSMF"
	}
	if c.Configuration.Sbi.Scheme == "" {
		c.Configuration.Sbi.Scheme = "http"
	}
	if c.Configuration.Sbi.BindingIPv4 == "" {
		c.Configuration.Sbi.BindingIPv4 = "127.0.0.31"
	}
	if c.Configuration.Sbi.Port == 0 {
		c.Configuration.Sbi.Port = 8000
	}
	if c.Configuration.Grpc.BindingIPv4 == "" {
		c.Configuration.Grpc.BindingIPv4 = "127.0.0.31"
	}
	if c.Configuration.Grpc.Port == 0 {
		c.Configuration.Grpc.Port = 50070
	}
	if c.Configuration.DefaultProtocols.Transport == "" {
		c.Configuration.DefaultProtocols.Transport = "HTTP2"
	}
	if c.Configuration.DefaultProtocols.Payload == "" {
		c.Configuration.DefaultProtocols.Payload = "JSON"
	}
	if c.Configuration.Task.TimeoutSeconds <= 0 {
		c.Configuration.Task.TimeoutSeconds = 30
	}
	if c.Configuration.Task.Storage.BackendType == "" {
		c.Configuration.Task.Storage.BackendType = "filesystem"
	}
	if c.Configuration.Task.Storage.ObjectPrefix == "" {
		c.Configuration.Task.Storage.ObjectPrefix = "results"
	}
	if c.Logger.Level == "" {
		c.Logger.Level = "info"
	}
}

func (c *Config) SbiAddr() string {
	return fmt.Sprintf("%s:%d", c.Configuration.Sbi.BindingIPv4, c.Configuration.Sbi.Port)
}

func (c *Config) GRPCAddr() string {
	return fmt.Sprintf("%s:%d", c.Configuration.Grpc.BindingIPv4, c.Configuration.Grpc.Port)
}
