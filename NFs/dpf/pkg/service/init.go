package service

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/sirupsen/logrus"

	dpf_context "github.com/free5gc/dpf/internal/context"
	"github.com/free5gc/dpf/internal/ingress"
	"github.com/free5gc/dpf/internal/logger"
	"github.com/free5gc/dpf/internal/processor"
	"github.com/free5gc/dpf/internal/rpc"
	"github.com/free5gc/dpf/pkg/app"
	"github.com/free5gc/dpf/pkg/factory"
)

type DpfApp struct {
	cfg           *factory.Config
	ctx           context.Context
	cancel        context.CancelFunc
	dpfCtx        *dpf_context.Context
	processor     *processor.Processor
	rpcServer     *rpc.Server
	ingressServer *ingress.Server
	wg            sync.WaitGroup
}

var _ app.App = (*DpfApp)(nil)

func NewApp(parent context.Context, cfg *factory.Config) (*DpfApp, error) {
	if err := logger.Configure(cfg.Logger.Enable, cfg.Logger.Level, cfg.Logger.ReportCaller); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	dpfCtx := dpf_context.New(cfg)
	proc := processor.New(cfg, dpfCtx)
	var ingressServer *ingress.Server
	if cfg.Configuration.RanIngress.Enable {
		http3Addr := ""
		if cfg.Configuration.RanIngress.HTTP3.Enable {
			http3Addr = cfg.RanIngressHTTP3Addr()
		}
		quicAddr := ""
		if cfg.Configuration.RanIngress.QUIC.Enable {
			quicAddr = cfg.RanIngressQUICAddr()
		}
		ingressServer = ingress.New(
			cfg.RanIngressAddr(),
			http3Addr,
			quicAddr,
			cfg.Configuration.RanIngress.TLS.CertFile,
			cfg.Configuration.RanIngress.TLS.KeyFile,
			cfg.Configuration.RanIngress.MaxBodyBytes,
			proc,
		)
	}
	return &DpfApp{
		cfg:           cfg,
		ctx:           ctx,
		cancel:        cancel,
		dpfCtx:        dpfCtx,
		processor:     proc,
		rpcServer:     rpc.New(dpfCtx.GRPCAddr, proc),
		ingressServer: ingressServer,
	}, nil
}

func (a *DpfApp) Start() {
	if err := a.rpcServer.Run(a.ctx, &a.wg); err != nil {
		logger.MainLog.Fatalf("failed to start DPF gRPC server: %v", err)
	}
	if a.ingressServer != nil {
		if err := a.ingressServer.Run(a.ctx, &a.wg); err != nil {
			logger.MainLog.Fatalf("failed to start DPF RAN ingress HTTP server: %v", err)
		}
	}
	<-a.ctx.Done()
	a.wg.Wait()
}

func (a *DpfApp) SetLogEnable(enable bool) {
	if enable {
		logger.Log.SetOutput(os.Stderr)
	} else {
		logger.Log.SetOutput(io.Discard)
	}
}

func (a *DpfApp) SetLogLevel(level string) {
	parsedLevel, err := logrus.ParseLevel(level)
	if err == nil {
		logger.Log.SetLevel(parsedLevel)
	}
}

func (a *DpfApp) SetReportCaller(reportCaller bool) {
	logger.Log.SetReportCaller(reportCaller)
}

func (a *DpfApp) Terminate() {
	a.cancel()
	if a.ingressServer != nil {
		a.ingressServer.Stop()
	}
	a.rpcServer.Stop()
}

func (a *DpfApp) Context() *dpf_context.Context {
	return a.dpfCtx
}

func (a *DpfApp) Config() *factory.Config {
	return a.cfg
}
