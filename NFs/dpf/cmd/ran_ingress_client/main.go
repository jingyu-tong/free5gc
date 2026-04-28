package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/http2"
)

const quicALPN = "free5gc-dpf-ran-ingress"

func main() {
	protocol := flag.String("protocol", "HTTP2", "HTTP2, HTTP3, or QUIC")
	url := flag.String("url", "http://127.0.0.32:8071/v1/ran/csi/latest", "RAN ingress URL")
	packetID := flag.String("packet-id", "latest", "QUIC packet id")
	input := flag.String("input", "", "packet file")
	timeout := flag.Duration("timeout", 10*time.Second, "request timeout")
	flag.Parse()

	if *input == "" {
		fatalf("-input is required")
	}
	body, err := os.ReadFile(*input)
	if err != nil {
		fatalf("read input: %v", err)
	}

	switch strings.ToUpper(*protocol) {
	case "HTTP2":
		err = postHTTP2(*url, body, *timeout)
	case "HTTP3":
		err = postHTTP3(*url, body, *timeout)
	case "QUIC":
		err = postQUIC(*url, *packetID, body, *timeout)
	default:
		err = fmt.Errorf("unsupported protocol %q", *protocol)
	}
	if err != nil {
		fatalf("%v", err)
	}
}

func postHTTP2(rawURL string, body []byte, timeout time.Duration) error {
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			dialer := &net.Dialer{}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	client := &http.Client{Transport: transport, Timeout: timeout}
	return postHTTP(client, rawURL, body)
}

func postHTTP3(rawURL string, body []byte, timeout time.Duration) error {
	transport := &http3.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	defer transport.Close()
	client := &http.Client{Transport: transport, Timeout: timeout}
	return postHTTP(client, rawURL, body)
}

func postHTTP(client *http.Client, rawURL string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func postQUIC(target, packetID string, body []byte, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := quic.DialAddr(ctx, hostPortFromURL(target), &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{quicALPN},
	}, nil)
	if err != nil {
		return err
	}
	defer conn.CloseWithError(0, "")
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return err
	}
	header, err := json.Marshal(map[string]string{"packetId": packetID})
	if err != nil {
		return err
	}
	if _, err := stream.Write(append(header, '\n')); err != nil {
		return err
	}
	if _, err := stream.Write(body); err != nil {
		return err
	}
	if err := stream.Close(); err != nil {
		return err
	}
	ack, err := io.ReadAll(stream)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(ack), "OK") {
		return fmt.Errorf("QUIC ingress failed: %s", strings.TrimSpace(string(ack)))
	}
	return nil
}

func hostPortFromURL(raw string) string {
	raw = strings.TrimPrefix(raw, "quic://")
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "https://")
	if idx := strings.Index(raw, "/"); idx >= 0 {
		raw = raw[:idx]
	}
	return raw
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
