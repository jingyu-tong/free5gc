package context

import "github.com/free5gc/dpf/pkg/factory"

type Context struct {
	DpfID    string
	GRPCAddr string
}

func New(cfg *factory.Config) *Context {
	return &Context{
		DpfID:    cfg.Configuration.DpfID,
		GRPCAddr: cfg.GRPCAddr(),
	}
}
