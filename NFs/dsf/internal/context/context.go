package context

import "github.com/free5gc/dsf/pkg/factory"

type Context struct {
	DsfID    string
	GRPCAddr string
	RootDir  string
}

func New(cfg *factory.Config) *Context {
	return &Context{
		DsfID:    cfg.Configuration.DsfID,
		GRPCAddr: cfg.GRPCAddr(),
		RootDir:  cfg.Configuration.Storage.RootDir,
	}
}
