package dataplane

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"

	"github.com/free5gc/dsf/internal/logger"
	"github.com/free5gc/dsf/internal/processor"
)

const quicALPN = "free5gc-dsf-quic"

type Server struct {
	http3Addr string
	quicAddr  string
	certFile  string
	keyFile   string
	proc      *processor.Processor

	http3Server *http3.Server
	quicLn      *quic.Listener
}

type quicHeader struct {
	TransferSessionID string `json:"transferSessionId"`
}

func New(http3Addr, quicAddr, certFile, keyFile string, proc *processor.Processor) *Server {
	return &Server{
		http3Addr: http3Addr,
		quicAddr:  quicAddr,
		certFile:  certFile,
		keyFile:   keyFile,
		proc:      proc,
	}
}

func (s *Server) Run(ctx context.Context, wg *sync.WaitGroup) error {
	if s.http3Addr != "" {
		if err := s.runHTTP3(ctx, wg); err != nil {
			return err
		}
	}
	if s.quicAddr != "" {
		if err := s.runQUIC(ctx, wg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Stop() {
	if s.http3Server != nil {
		_ = s.http3Server.Close()
	}
	if s.quicLn != nil {
		_ = s.quicLn.Close()
	}
}

func (s *Server) runHTTP3(ctx context.Context, wg *sync.WaitGroup) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/result/", s.handleHTTP3Result)
	server := &http3.Server{
		Addr:    s.http3Addr,
		Handler: mux,
	}
	s.http3Server = server

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		_ = server.Close()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.MainLog.Infof("DSF HTTP/3 data plane listening on %s", s.http3Addr)
		if err := server.ListenAndServeTLS(s.certFile, s.keyFile); err != nil && ctx.Err() == nil {
			logger.MainLog.Errorf("DSF HTTP/3 data plane stopped: %v", err)
		}
	}()
	return nil
}

func (s *Server) handleHTTP3Result(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/v1/result/")
	if sessionID == "" {
		http.Error(w, "missing transfer session id", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	resp, err := s.proc.StoreNativePayload(r.Context(), sessionID, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if resp.State != "COMPLETED" {
		http.Error(w, resp.Message, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) runQUIC(ctx context.Context, wg *sync.WaitGroup) error {
	cert, err := tls.LoadX509KeyPair(s.certFile, s.keyFile)
	if err != nil {
		return err
	}
	ln, err := quic.ListenAddr(s.quicAddr, &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{quicALPN},
	}, nil)
	if err != nil {
		return err
	}
	s.quicLn = ln

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		_ = ln.Close()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.MainLog.Infof("DSF QUIC data plane listening on %s", s.quicAddr)
		for {
			conn, err := ln.Accept(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.MainLog.Warnf("DSF QUIC accept failed: %v", err)
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.handleQUICConnection(ctx, conn)
			}()
		}
	}()
	return nil
}

func (s *Server) handleQUICConnection(ctx context.Context, conn *quic.Conn) {
	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			return
		}
		go s.handleQUICStream(ctx, stream)
	}
}

func (s *Server) handleQUICStream(ctx context.Context, stream *quic.Stream) {
	defer stream.Close()

	reader := bufio.NewReader(stream)
	headerLine, err := reader.ReadBytes('\n')
	if err != nil {
		_, _ = stream.Write([]byte(fmt.Sprintf("ERROR %v\n", err)))
		return
	}
	var header quicHeader
	if err := json.Unmarshal(headerLine, &header); err != nil {
		_, _ = stream.Write([]byte(fmt.Sprintf("ERROR %v\n", err)))
		return
	}
	if header.TransferSessionID == "" {
		_, _ = stream.Write([]byte("ERROR missing transferSessionId\n"))
		return
	}

	resultCh := make(chan struct {
		err error
	}, 1)
	go func() {
		_, err := s.proc.StoreNativePayload(ctx, header.TransferSessionID, reader)
		resultCh <- struct{ err error }{err: err}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			_, _ = stream.Write([]byte(fmt.Sprintf("ERROR %v\n", result.err)))
			return
		}
		_, _ = stream.Write([]byte("OK\n"))
	case <-time.After(60 * time.Second):
		_, _ = stream.Write([]byte("ERROR timeout\n"))
	case <-ctx.Done():
		_, _ = stream.Write([]byte("ERROR shutdown\n"))
	}
}

var _ io.Reader = (*bufio.Reader)(nil)
