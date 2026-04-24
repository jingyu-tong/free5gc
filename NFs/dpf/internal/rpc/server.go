package rpc

import (
	"context"
	"errors"
	"net"
	"sync"

	"google.golang.org/grpc"

	"github.com/free5gc/dataapi/pkg/api"
	"github.com/free5gc/dpf/internal/logger"
	"github.com/free5gc/dpf/internal/processor"
)

type Server struct {
	addr   string
	server *grpc.Server
}

func New(addr string, proc *processor.Processor) *Server {
	s := grpc.NewServer()
	api.RegisterDpfControlServiceServer(s, proc)
	return &Server{
		addr:   addr,
		server: s,
	}
}

func (s *Server) Run(ctx context.Context, wg *sync.WaitGroup) error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		go func() {
			<-ctx.Done()
			s.server.GracefulStop()
		}()
		logger.RPCLog.Infof("start DPF gRPC server on %s", s.addr)
		if err := s.server.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.RPCLog.Errorf("DPF gRPC server stopped with error: %v", err)
		}
	}()
	return nil
}

func (s *Server) Stop() {
	s.server.GracefulStop()
}
