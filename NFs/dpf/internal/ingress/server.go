package ingress

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/free5gc/dpf/internal/logger"
	"github.com/free5gc/dpf/internal/processor"
)

const quicALPN = "free5gc-dpf-ran-ingress"

type Server struct {
	addr         string
	http3Addr    string
	quicAddr     string
	certFile     string
	keyFile      string
	maxBodyBytes int64
	processor    *processor.Processor
	server       *http.Server
	http3Server  *http3.Server
	listener     net.Listener
	quicLn       *quic.Listener
}

func New(addr, http3Addr, quicAddr, certFile, keyFile string, maxBodyBytes int64, proc *processor.Processor) *Server {
	s := &Server{
		addr:         addr,
		http3Addr:    http3Addr,
		quicAddr:     quicAddr,
		certFile:     certFile,
		keyFile:      keyFile,
		maxBodyBytes: maxBodyBytes,
		processor:    proc,
	}
	mux := s.newMux()
	s.server = &http.Server{
		Addr:              addr,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/ran/csi/", s.handleCSI)
	return mux
}

func (s *Server) Run(ctx context.Context, wg *sync.WaitGroup) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = ln

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			logger.MainLog.Warnf("failed to shutdown DPF RAN ingress HTTP/2 server: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.MainLog.Infof("start DPF RAN ingress HTTP/2 h2c server on %s", s.addr)
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.MainLog.Errorf("DPF RAN ingress HTTP/2 server stopped with error: %v", err)
		}
	}()

	if s.http3Addr != "" {
		s.runHTTP3(ctx, wg)
	}
	if s.quicAddr != "" {
		if err := s.runQUIC(ctx, wg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Stop() {
	if s.server != nil {
		_ = s.server.Close()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	if s.http3Server != nil {
		_ = s.http3Server.Close()
	}
	if s.quicLn != nil {
		_ = s.quicLn.Close()
	}
}

func (s *Server) runHTTP3(ctx context.Context, wg *sync.WaitGroup) {
	server := &http3.Server{Addr: s.http3Addr, Handler: s.newMux()}
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
		logger.MainLog.Infof("start DPF RAN ingress HTTP/3 server on %s", s.http3Addr)
		if err := server.ListenAndServeTLS(s.certFile, s.keyFile); err != nil && ctx.Err() == nil {
			logger.MainLog.Errorf("DPF RAN ingress HTTP/3 server stopped with error: %v", err)
		}
	}()
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
		logger.MainLog.Infof("start DPF RAN ingress QUIC server on %s", s.quicAddr)
		for {
			conn, err := ln.Accept(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.MainLog.Warnf("DPF RAN ingress QUIC accept failed: %v", err)
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
		go s.handleQUICStream(stream)
	}
}

func (s *Server) handleQUICStream(stream *quic.Stream) {
	defer stream.Close()
	reader := bufio.NewReader(io.LimitReader(stream, s.maxBodyBytes+4096))
	headerLine, err := reader.ReadBytes('\n')
	if err != nil {
		_, _ = stream.Write([]byte(fmt.Sprintf("ERROR %v\n", err)))
		return
	}
	var header struct {
		PacketID string `json:"packetId"`
	}
	if err := json.Unmarshal(headerLine, &header); err != nil {
		_, _ = stream.Write([]byte(fmt.Sprintf("ERROR %v\n", err)))
		return
	}
	if header.PacketID == "" {
		header.PacketID = "latest"
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		_, _ = stream.Write([]byte(fmt.Sprintf("ERROR %v\n", err)))
		return
	}
	if int64(len(body)) > s.maxBodyBytes {
		_, _ = stream.Write([]byte("ERROR body too large\n"))
		return
	}
	if len(body) == 0 {
		_, _ = stream.Write([]byte("ERROR empty CSI packet\n"))
		return
	}
	s.processor.StoreRANPacket(header.PacketID, body)
	_, _ = stream.Write([]byte("OK\n"))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleCSI(w http.ResponseWriter, r *http.Request) {
	packetID, err := packetIDFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost, http.MethodPut:
		s.storePacket(w, r, packetID)
	case http.MethodGet:
		s.loadPacket(w, packetID)
	default:
		w.Header().Set("Allow", "GET, POST, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) storePacket(w http.ResponseWriter, r *http.Request, packetID string) {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, s.maxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body failed: %v", err), http.StatusBadRequest)
		return
	}
	if int64(len(body)) > s.maxBodyBytes {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}
	if len(body) == 0 {
		http.Error(w, "empty CSI packet", http.StatusBadRequest)
		return
	}

	s.processor.StoreRANPacket(packetID, body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"packetId": packetID,
		"bytes":    len(body),
		"state":    "STORED",
	})
}

func (s *Server) loadPacket(w http.ResponseWriter, packetID string) {
	body, receivedAt, ok := s.processor.LoadRANPacket(packetID)
	if !ok {
		http.Error(w, "CSI packet not found", http.StatusNotFound)
		return
	}
	logger.ProcLog.WithFields(map[string]any{
		"packet_id":   packetID,
		"bytes":       len(body),
		"received_at": receivedAt.Format(time.RFC3339Nano),
	}).Infof("DPF_RAN_INGRESS_FETCHED")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-RAN-CSI-Received-At", receivedAt.Format(time.RFC3339Nano))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func packetIDFromPath(path string) (string, error) {
	const prefix = "/v1/ran/csi/"
	if !strings.HasPrefix(path, prefix) {
		return "", fmt.Errorf("unsupported path")
	}
	raw := strings.TrimPrefix(path, prefix)
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return "", fmt.Errorf("packet id is required")
	}
	packetID, err := url.PathUnescape(raw)
	if err != nil {
		return "", fmt.Errorf("invalid packet id")
	}
	if strings.Contains(packetID, "/") {
		return "", fmt.Errorf("packet id must be a single path segment")
	}
	return packetID, nil
}
