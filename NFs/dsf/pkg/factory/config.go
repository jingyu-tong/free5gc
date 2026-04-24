package factory

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultConfigPath = "./config/dsfcfg.yaml"

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
	DsfID        string       `yaml:"dsfId"`
	Grpc         Grpc         `yaml:"grpc"`
	Storage      Storage      `yaml:"storage"`
}

type Grpc struct {
	BindingIPv4 string `yaml:"bindingIPv4"`
	Port        int    `yaml:"port"`
}

type Storage struct {
	RootDir string `yaml:"rootDir"`
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
	if c.Configuration.DsfID == "" {
		c.Configuration.DsfID = "dsf-1"
	}
	if c.Configuration.Grpc.BindingIPv4 == "" {
		c.Configuration.Grpc.BindingIPv4 = "127.0.0.33"
	}
	if c.Configuration.Grpc.Port == 0 {
		c.Configuration.Grpc.Port = 50072
	}
	if c.Configuration.Storage.RootDir == "" {
		c.Configuration.Storage.RootDir = "./data/dsf"
	}
	if c.Logger.Level == "" {
		c.Logger.Level = "info"
	}
}

func (c *Config) GRPCAddr() string {
	return fmt.Sprintf("%s:%d", c.Configuration.Grpc.BindingIPv4, c.Configuration.Grpc.Port)
}
