package factory

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultConfigPath = "./config/dpfcfg.yaml"

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
	DpfID                 string       `yaml:"dpfId"`
	Grpc                  Grpc         `yaml:"grpc"`
	ChunkSize             int          `yaml:"chunkSize"`
	HTTPSourceTimeoutSecs int          `yaml:"httpSourceTimeoutSeconds"`
}

type Grpc struct {
	BindingIPv4 string `yaml:"bindingIPv4"`
	Port        int    `yaml:"port"`
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
	if c.Configuration.DpfID == "" {
		c.Configuration.DpfID = "dpf-1"
	}
	if c.Configuration.Grpc.BindingIPv4 == "" {
		c.Configuration.Grpc.BindingIPv4 = "127.0.0.32"
	}
	if c.Configuration.Grpc.Port == 0 {
		c.Configuration.Grpc.Port = 50071
	}
	if c.Configuration.ChunkSize <= 0 {
		c.Configuration.ChunkSize = 32768
	}
	if c.Configuration.HTTPSourceTimeoutSecs <= 0 {
		c.Configuration.HTTPSourceTimeoutSecs = 15
	}
	if c.Logger.Level == "" {
		c.Logger.Level = "info"
	}
}

func (c *Config) GRPCAddr() string {
	return fmt.Sprintf("%s:%d", c.Configuration.Grpc.BindingIPv4, c.Configuration.Grpc.Port)
}
