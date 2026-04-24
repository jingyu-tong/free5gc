package sbi

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/free5gc/dsmf/internal/logger"
	"github.com/free5gc/dsmf/internal/processor"
)

type Server struct {
	addr       string
	processor  *processor.Processor
	httpServer *http.Server
}

func New(addr string, processor *processor.Processor) *Server {
	router := gin.New()
	router.Use(gin.Recovery())

	s := &Server{
		addr:      addr,
		processor: processor,
	}

	group := router.Group("/ndsmf-data-service/v1")
	applyRoutes(group, s.routes())
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: router,
	}
	return s
}

func (s *Server) Run(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	go func() {
		defer wg.Done()
		go func() {
			<-ctx.Done()
			_ = s.httpServer.Shutdown(context.Background())
		}()
		logger.SBILog.Infof("start DSMF HTTP server on %s", s.addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.SBILog.Errorf("DSMF HTTP server stopped with error: %v", err)
		}
	}()
	return nil
}

func (s *Server) Stop() {
	_ = s.httpServer.Shutdown(context.Background())
}
