package context

import "github.com/free5gc/dsmf/pkg/factory"

type Context struct {
	Name     string
	SbiAddr  string
	GRPCAddr string
}

func New(cfg *factory.Config) *Context {
	return &Context{
		Name:     cfg.Configuration.DsmfName,
		SbiAddr:  cfg.SbiAddr(),
		GRPCAddr: cfg.GRPCAddr(),
	}
}
