package app

import (
	dsf_context "github.com/free5gc/dsf/internal/context"
	"github.com/free5gc/dsf/pkg/factory"
)

type App interface {
	SetLogEnable(bool)
	SetLogLevel(string)
	SetReportCaller(bool)
	Start()
	Terminate()
	Context() *dsf_context.Context
	Config() *factory.Config
}
