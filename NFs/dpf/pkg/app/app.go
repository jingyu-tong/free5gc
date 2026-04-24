package app

import (
	dpf_context "github.com/free5gc/dpf/internal/context"
	"github.com/free5gc/dpf/pkg/factory"
)

type App interface {
	SetLogEnable(bool)
	SetLogLevel(string)
	SetReportCaller(bool)
	Start()
	Terminate()
	Context() *dpf_context.Context
	Config() *factory.Config
}
