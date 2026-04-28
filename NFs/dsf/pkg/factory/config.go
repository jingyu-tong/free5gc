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
	DsfID     string    `yaml:"dsfId"`
	Grpc      Grpc      `yaml:"grpc"`
	DataPlane DataPlane `yaml:"dataPlane"`
	Storage   Storage   `yaml:"storage"`
}

type Grpc struct {
	BindingIPv4 string `yaml:"bindingIPv4"`
	Port        int    `yaml:"port"`
}

type DataPlane struct {
	HTTP3 DataPlaneEndpoint `yaml:"http3"`
	QUIC  DataPlaneEndpoint `yaml:"quic"`
	TLS   TLSConfig         `yaml:"tls"`
}

type DataPlaneEndpoint struct {
	Enable      bool   `yaml:"enable"`
	BindingIPv4 string `yaml:"bindingIPv4"`
	Port        int    `yaml:"port"`
}

type TLSConfig struct {
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
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
	if c.Configuration.DataPlane.HTTP3.BindingIPv4 == "" {
		c.Configuration.DataPlane.HTTP3.BindingIPv4 = c.Configuration.Grpc.BindingIPv4
	}
	if c.Configuration.DataPlane.HTTP3.Port == 0 {
		c.Configuration.DataPlane.HTTP3.Port = 50073
	}
	if c.Configuration.DataPlane.QUIC.BindingIPv4 == "" {
		c.Configuration.DataPlane.QUIC.BindingIPv4 = c.Configuration.Grpc.BindingIPv4
	}
	if c.Configuration.DataPlane.QUIC.Port == 0 {
		c.Configuration.DataPlane.QUIC.Port = 50074
	}
	if c.Configuration.DataPlane.TLS.CertFile == "" {
		c.Configuration.DataPlane.TLS.CertFile = "./cert/nrf.pem"
	}
	if c.Configuration.DataPlane.TLS.KeyFile == "" {
		c.Configuration.DataPlane.TLS.KeyFile = "./cert/nrf.key"
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

func (c *Config) HTTP3Addr() string {
	return fmt.Sprintf("%s:%d", c.Configuration.DataPlane.HTTP3.BindingIPv4, c.Configuration.DataPlane.HTTP3.Port)
}

func (c *Config) QUICAddr() string {
	return fmt.Sprintf("%s:%d", c.Configuration.DataPlane.QUIC.BindingIPv4, c.Configuration.DataPlane.QUIC.Port)
}
