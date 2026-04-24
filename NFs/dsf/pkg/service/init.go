package service

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/sirupsen/logrus"

	dsf_context "github.com/free5gc/dsf/internal/context"
	"github.com/free5gc/dsf/internal/logger"
	"github.com/free5gc/dsf/internal/processor"
	"github.com/free5gc/dsf/internal/rpc"
	"github.com/free5gc/dsf/pkg/app"
	"github.com/free5gc/dsf/pkg/factory"
)

type DsfApp struct {
	cfg       *factory.Config
	ctx       context.Context
	cancel    context.CancelFunc
	dsfCtx    *dsf_context.Context
	processor *processor.Processor
	rpcServer *rpc.Server
	wg        sync.WaitGroup
}

var _ app.App = (*DsfApp)(nil)

func NewApp(parent context.Context, cfg *factory.Config) (*DsfApp, error) {
	if err := logger.Configure(cfg.Logger.Enable, cfg.Logger.Level, cfg.Logger.ReportCaller); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	dsfCtx := dsf_context.New(cfg)
	proc := processor.New(cfg, dsfCtx)
	return &DsfApp{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		dsfCtx:    dsfCtx,
		processor: proc,
		rpcServer: rpc.New(dsfCtx.GRPCAddr, proc),
	}, nil
}

func (a *DsfApp) Start() {
	if err := a.rpcServer.Run(a.ctx, &a.wg); err != nil {
		logger.MainLog.Fatalf("failed to start DSF gRPC server: %v", err)
	}
	<-a.ctx.Done()
	a.wg.Wait()
}

func (a *DsfApp) SetLogEnable(enable bool) {
	if enable {
		logger.Log.SetOutput(os.Stderr)
	} else {
		logger.Log.SetOutput(io.Discard)
	}
}

func (a *DsfApp) SetLogLevel(level string) {
	parsedLevel, err := logrus.ParseLevel(level)
	if err == nil {
		logger.Log.SetLevel(parsedLevel)
	}
}

func (a *DsfApp) SetReportCaller(reportCaller bool) {
	logger.Log.SetReportCaller(reportCaller)
}

func (a *DsfApp) Terminate() {
	a.cancel()
	a.rpcServer.Stop()
}

func (a *DsfApp) Context() *dsf_context.Context {
	return a.dsfCtx
}

func (a *DsfApp) Config() *factory.Config {
	return a.cfg
}
