package context

import "github.com/free5gc/dsf/pkg/factory"

type Context struct {
	DsfID         string
	GRPCAddr      string
	HTTP3Addr     string
	QUICAddr      string
	DataPlaneCert string
	DataPlaneKey  string
	RootDir       string
}

func New(cfg *factory.Config) *Context {
	return &Context{
		DsfID:         cfg.Configuration.DsfID,
		GRPCAddr:      cfg.GRPCAddr(),
		HTTP3Addr:     cfg.HTTP3Addr(),
		QUICAddr:      cfg.QUICAddr(),
		DataPlaneCert: cfg.Configuration.DataPlane.TLS.CertFile,
		DataPlaneKey:  cfg.Configuration.DataPlane.TLS.KeyFile,
		RootDir:       cfg.Configuration.Storage.RootDir,
	}
}
