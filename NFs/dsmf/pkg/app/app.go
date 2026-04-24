package app

import (
	dsmf_context "github.com/free5gc/dsmf/internal/context"
	"github.com/free5gc/dsmf/pkg/factory"
)

type App interface {
	SetLogEnable(bool)
	SetLogLevel(string)
	SetReportCaller(bool)
	Start()
	Terminate()
	Context() *dsmf_context.Context
	Config() *factory.Config
}
