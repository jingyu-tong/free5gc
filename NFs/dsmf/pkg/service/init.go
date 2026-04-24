package service

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/sirupsen/logrus"

	dsmf_context "github.com/free5gc/dsmf/internal/context"
	"github.com/free5gc/dsmf/internal/logger"
	"github.com/free5gc/dsmf/internal/processor"
	"github.com/free5gc/dsmf/internal/rpc"
	"github.com/free5gc/dsmf/internal/sbi"
	"github.com/free5gc/dsmf/pkg/app"
	"github.com/free5gc/dsmf/pkg/factory"
)

type DsmfApp struct {
	cfg       *factory.Config
	ctx       context.Context
	cancel    context.CancelFunc
	dsmfCtx   *dsmf_context.Context
	processor *processor.Processor
	rpcServer *rpc.Server
	sbiServer *sbi.Server
	wg        sync.WaitGroup
}

var _ app.App = (*DsmfApp)(nil)

func NewApp(parent context.Context, cfg *factory.Config) (*DsmfApp, error) {
	if err := logger.Configure(cfg.Logger.Enable, cfg.Logger.Level, cfg.Logger.ReportCaller); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	dsmfCtx := dsmf_context.New(cfg)
	proc := processor.New(cfg, dsmfCtx)
	return &DsmfApp{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		dsmfCtx:   dsmfCtx,
		processor: proc,
		rpcServer: rpc.New(dsmfCtx.GRPCAddr, proc),
		sbiServer: sbi.New(dsmfCtx.SbiAddr, proc),
	}, nil
}

func (a *DsmfApp) Start() {
	if err := a.rpcServer.Run(a.ctx, &a.wg); err != nil {
		logger.MainLog.Fatalf("failed to start DSMF gRPC server: %v", err)
	}
	if err := a.sbiServer.Run(a.ctx, &a.wg); err != nil {
		logger.MainLog.Fatalf("failed to start DSMF HTTP server: %v", err)
	}
	<-a.ctx.Done()
	a.wg.Wait()
}

func (a *DsmfApp) SetLogEnable(enable bool) {
	if enable {
		logger.Log.SetOutput(os.Stderr)
	} else {
		logger.Log.SetOutput(io.Discard)
	}
}

func (a *DsmfApp) SetLogLevel(level string) {
	parsedLevel, err := logrus.ParseLevel(level)
	if err == nil {
		logger.Log.SetLevel(parsedLevel)
	}
}

func (a *DsmfApp) SetReportCaller(reportCaller bool) {
	logger.Log.SetReportCaller(reportCaller)
}

func (a *DsmfApp) Terminate() {
	a.cancel()
	a.rpcServer.Stop()
	a.sbiServer.Stop()
}

func (a *DsmfApp) Context() *dsmf_context.Context {
	return a.dsmfCtx
}

func (a *DsmfApp) Config() *factory.Config {
	return a.cfg
}
