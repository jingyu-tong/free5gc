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
	DpfID                 string     `yaml:"dpfId"`
	Grpc                  Grpc       `yaml:"grpc"`
	RanIngress            RanIngress `yaml:"ranIngress"`
	ChunkSize             int        `yaml:"chunkSize"`
	HTTPSourceTimeoutSecs int        `yaml:"httpSourceTimeoutSeconds"`
	HTTPSourceRetry       Retry      `yaml:"httpSourceRetry"`
}

type Grpc struct {
	BindingIPv4 string `yaml:"bindingIPv4"`
	Port        int    `yaml:"port"`
}

type RanIngress struct {
	Enable       bool               `yaml:"enable"`
	BindingIPv4  string             `yaml:"bindingIPv4"`
	Port         int                `yaml:"port"`
	HTTP3        RanIngressEndpoint `yaml:"http3"`
	QUIC         RanIngressEndpoint `yaml:"quic"`
	TLS          TLSConfig          `yaml:"tls"`
	MaxBodyBytes int64              `yaml:"maxBodyBytes"`
}

type RanIngressEndpoint struct {
	Enable      bool   `yaml:"enable"`
	BindingIPv4 string `yaml:"bindingIPv4"`
	Port        int    `yaml:"port"`
}

type TLSConfig struct {
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

type Retry struct {
	MaxAttempts    int `yaml:"maxAttempts"`
	IntervalMillis int `yaml:"intervalMillis"`
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
	if c.Configuration.HTTPSourceRetry.MaxAttempts <= 0 {
		c.Configuration.HTTPSourceRetry.MaxAttempts = 1
	}
	if c.Configuration.HTTPSourceRetry.IntervalMillis <= 0 {
		c.Configuration.HTTPSourceRetry.IntervalMillis = 200
	}
	if c.Configuration.RanIngress.BindingIPv4 == "" {
		c.Configuration.RanIngress.BindingIPv4 = "127.0.0.32"
	}
	if c.Configuration.RanIngress.Port == 0 {
		c.Configuration.RanIngress.Port = 8071
	}
	if c.Configuration.RanIngress.HTTP3.BindingIPv4 == "" {
		c.Configuration.RanIngress.HTTP3.BindingIPv4 = c.Configuration.RanIngress.BindingIPv4
	}
	if c.Configuration.RanIngress.HTTP3.Port == 0 {
		c.Configuration.RanIngress.HTTP3.Port = 8072
	}
	if c.Configuration.RanIngress.QUIC.BindingIPv4 == "" {
		c.Configuration.RanIngress.QUIC.BindingIPv4 = c.Configuration.RanIngress.BindingIPv4
	}
	if c.Configuration.RanIngress.QUIC.Port == 0 {
		c.Configuration.RanIngress.QUIC.Port = 8073
	}
	if c.Configuration.RanIngress.TLS.CertFile == "" {
		c.Configuration.RanIngress.TLS.CertFile = "./cert/nrf.pem"
	}
	if c.Configuration.RanIngress.TLS.KeyFile == "" {
		c.Configuration.RanIngress.TLS.KeyFile = "./cert/nrf.key"
	}
	if c.Configuration.RanIngress.MaxBodyBytes <= 0 {
		c.Configuration.RanIngress.MaxBodyBytes = 16 * 1024 * 1024
	}
	if c.Logger.Level == "" {
		c.Logger.Level = "info"
	}
}

func (c *Config) GRPCAddr() string {
	return fmt.Sprintf("%s:%d", c.Configuration.Grpc.BindingIPv4, c.Configuration.Grpc.Port)
}

func (c *Config) RanIngressAddr() string {
	return fmt.Sprintf("%s:%d", c.Configuration.RanIngress.BindingIPv4, c.Configuration.RanIngress.Port)
}

func (c *Config) RanIngressHTTP3Addr() string {
	return fmt.Sprintf("%s:%d", c.Configuration.RanIngress.HTTP3.BindingIPv4, c.Configuration.RanIngress.HTTP3.Port)
}

func (c *Config) RanIngressQUICAddr() string {
	return fmt.Sprintf("%s:%d", c.Configuration.RanIngress.QUIC.BindingIPv4, c.Configuration.RanIngress.QUIC.Port)
}
