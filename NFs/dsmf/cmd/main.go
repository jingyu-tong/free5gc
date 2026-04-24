package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/free5gc/dsmf/internal/logger"
	"github.com/free5gc/dsmf/pkg/factory"
	"github.com/free5gc/dsmf/pkg/service"
	logger_util "github.com/free5gc/util/logger"
	"github.com/free5gc/util/version"
)

func main() {
	defer func() {
		if p := recover(); p != nil {
			logger.MainLog.Fatalf("panic: %v\n%s", p, string(debug.Stack()))
		}
	}()

	app := cli.NewApp()
	app.Name = "dsmf"
	app.Usage = "Data Service Management Function"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Load configuration from FILE",
		},
		&cli.StringSliceFlag{
			Name:    "log",
			Aliases: []string{"l"},
			Usage:   "Output NF log to FILE",
		},
	}
	app.Action = func(cliCtx *cli.Context) error {
		cfg, err := factory.ReadConfig(cliCtx.String("config"))
		if err != nil {
			return err
		}
		factory.DsmfConfig = cfg

		if err := initLogFile(cliCtx.StringSlice("log")); err != nil {
			return err
		}

		logger.MainLog.Infoln("DSMF version: ", version.GetVersion())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		dsmf, err := service.NewApp(ctx, cfg)
		if err != nil {
			return err
		}
		dsmf.Start()
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		logger.MainLog.Errorf("DSMF run error: %v", err)
	}
}

func initLogFile(logNfPath []string) error {
	for _, path := range logNfPath {
		if err := logger_util.LogFileHook(logger.Log, path); err != nil {
			return err
		}
	}
	return nil
}
