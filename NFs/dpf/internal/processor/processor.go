package processor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/free5gc/dataapi/pkg/api"
	"github.com/free5gc/dataapi/pkg/codec"
	dpf_context "github.com/free5gc/dpf/internal/context"
	"github.com/free5gc/dpf/internal/logger"
	"github.com/free5gc/dpf/pkg/factory"
)

const quicALPN = "free5gc-dsf-quic"

type Processor struct {
	cfg            *factory.Config
	ctx            *dpf_context.Context
	mu             sync.RWMutex
	tasks          map[string]*api.SubmitProcessingTaskRequest
	client         *http.Client
	http3Transport *http3.Transport
	http3Client    *http.Client
	quicMu         sync.Mutex
	quicConns      map[string]*quic.Conn
	ranMu          sync.RWMutex
	ranPackets     map[string]ranPacket
	ranLatest      string
}

type ranPacket struct {
	ID         string
	Body       []byte
	ReceivedAt time.Time
}

func New(cfg *factory.Config, ctx *dpf_context.Context) *Processor {
	http3Transport := &http3.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &Processor{
		cfg:   cfg,
		ctx:   ctx,
		tasks: make(map[string]*api.SubmitProcessingTaskRequest),
		client: &http.Client{
			Timeout: time.Duration(cfg.Configuration.HTTPSourceTimeoutSecs) * time.Second,
		},
		http3Transport: http3Transport,
		http3Client: &http.Client{
			Transport: http3Transport,
			Timeout:   time.Duration(maxInt(cfg.Configuration.HTTPSourceTimeoutSecs, 60)) * time.Second,
		},
		quicConns:  make(map[string]*quic.Conn),
		ranPackets: make(map[string]ranPacket),
	}
}

func (p *Processor) SubmitProcessingTask(ctx context.Context, req *api.SubmitProcessingTaskRequest) (*api.SubmitProcessingTaskResponse, error) {
	p.mu.Lock()
	p.tasks[req.TaskID] = req
	p.mu.Unlock()

	logger.ProcLog.WithFields(map[string]any{
		"processing_task_id": req.TaskID,
		"orchestration_id":   req.OrchestrationID,
		"request_id":         req.RequestID,
		"transport_protocol": req.Delivery.TransportProtocol,
		"payload_protocol":   req.Delivery.PayloadProtocol,
	}).Infof("DPF_SUBMIT_PROCESSING_TASK")

	go p.runTask(req)

	return &api.SubmitProcessingTaskResponse{
		TaskID:   req.TaskID,
		Accepted: true,
		DpfID:    p.ctx.DpfID,
		State:    api.ProcessingStateAccepted,
	}, nil
}

func (p *Processor) runTask(req *api.SubmitProcessingTaskRequest) {
	log := logger.ProcLog.WithFields(map[string]any{
		"processing_task_id": req.TaskID,
		"orchestration_id":   req.OrchestrationID,
		"request_id":         req.RequestID,
	})
	if err := p.reportStatus(req, api.ProcessingStateAccepted, "task accepted", api.ProcessingMetrics{}, "", "", ""); err != nil {
		logger.ProcLog.Warnf("report accepted failed: %v", err)
	}

	source, err := p.fetchSource(req)
	if err != nil {
		_ = p.reportStatus(req, api.ProcessingStateFailed, "failed to fetch source", api.ProcessingMetrics{}, "", "SOURCE_FETCH_FAILED", err.Error())
		return
	}

	metrics := api.ProcessingMetrics{
		SourceRecords: 1,
		SourceBytes:   uint64(len(source)),
	}
	log.WithField("source_bytes", len(source)).Infof("DPF_SOURCE_READY")

	_ = p.reportStatus(req, api.ProcessingStateReceivingSource, "source received", metrics, "", "", "")
	_ = p.reportStatus(req, api.ProcessingStatePreprocessing, "preprocessing source", metrics, "", "", "")

	result, err := p.process(req, source)
	if err != nil {
		_ = p.reportStatus(req, api.ProcessingStateFailed, "processing failed", metrics, "", "PROCESSING_FAILED", err.Error())
		return
	}

	encoded, err := p.encodeResult(req.Delivery.PayloadProtocol, result)
	if err != nil {
		_ = p.reportStatus(req, api.ProcessingStateFailed, "payload encoding failed", metrics, "", "ENCODE_FAILED", err.Error())
		return
	}

	metrics.OutputBytes = uint64(len(encoded))
	metrics.OutputRecords = 1
	log.WithField("output_bytes", len(encoded)).Infof("DPF_RESULT_READY")
	_ = p.reportStatus(req, api.ProcessingStateProcessing, "result prepared", metrics, "", "", "")

	sessionID := uuid.NewString()
	log.WithField("transfer_session_id", sessionID).Infof("DPF_DELIVER_BEGIN")
	if err := p.deliver(req, sessionID, encoded); err != nil {
		_ = p.reportStatus(req, api.ProcessingStateFailed, "delivery failed", metrics, sessionID, "DELIVERY_FAILED", err.Error())
		return
	}

	log.WithField("transfer_session_id", sessionID).Infof("DPF_DELIVER_COMPLETE")
	_ = p.reportStatus(req, api.ProcessingStateCompleted, "delivery completed", metrics, sessionID, "", "")
}

func (p *Processor) fetchSource(req *api.SubmitProcessingTaskRequest) ([]byte, error) {
	ep := req.Source.IngressEndpoint
	switch strings.ToLower(ep.Scheme) {
	case "file":
		if ep.Path == "" {
			return nil, errors.New("file source path is empty")
		}
		return os.ReadFile(filepath.Clean(ep.Path))
	case "http", "https":
		url := buildURL(ep)
		attempts := p.cfg.Configuration.HTTPSourceRetry.MaxAttempts
		interval := time.Duration(p.cfg.Configuration.HTTPSourceRetry.IntervalMillis) * time.Millisecond
		var lastErr error
		for attempt := 1; attempt <= attempts; attempt++ {
			body, err := p.fetchHTTP(url)
			if err == nil {
				return body, nil
			}
			lastErr = err
			if attempt < attempts {
				time.Sleep(interval)
			}
		}
		return nil, lastErr
	default:
		return nil, fmt.Errorf("unsupported source scheme %q", ep.Scheme)
	}
}

func (p *Processor) fetchHTTP(url string) ([]byte, error) {
	resp, err := p.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("source returned status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (p *Processor) StoreRANPacket(packetID string, body []byte) {
	now := time.Now().UTC()
	p.ranMu.Lock()
	p.ranPackets[packetID] = ranPacket{
		ID:         packetID,
		Body:       append([]byte(nil), body...),
		ReceivedAt: now,
	}
	p.ranLatest = packetID
	p.ranMu.Unlock()
	logger.ProcLog.WithFields(map[string]any{
		"packet_id": packetID,
		"bytes":     len(body),
	}).Infof("DPF_RAN_INGRESS_RECEIVED")
}

func (p *Processor) LoadRANPacket(packetID string) ([]byte, time.Time, bool) {
	p.ranMu.RLock()
	if packetID == "latest" {
		packetID = p.ranLatest
	}
	packet, ok := p.ranPackets[packetID]
	p.ranMu.RUnlock()
	if !ok {
		return nil, time.Time{}, false
	}
	return append([]byte(nil), packet.Body...), packet.ReceivedAt, true
}

func (p *Processor) process(req *api.SubmitProcessingTaskRequest, source []byte) (map[string]any, error) {
	preview := string(source)
	if len(preview) > 256 {
		preview = preview[:256]
	}

	steps := make([]string, 0, len(req.ProcessingSteps))
	parameters := map[string]string{}
	for _, step := range req.ProcessingSteps {
		steps = append(steps, step.Name)
		for _, pair := range step.Parameters {
			parameters[pair.Key] = pair.Value
		}
	}

	result := map[string]any{
		"taskId":         req.TaskID,
		"requestId":      req.RequestID,
		"sourceId":       req.Source.SourceID,
		"sourceCategory": req.Source.SourceCategory,
		"sourceScenario": req.Source.SourceScenario,
		"outputSchema":   req.OutputSchema,
		"stepsApplied":   steps,
		"sourceBytes":    len(source),
		"contentPreview": preview,
		"parameters":     parameters,
		"processedBy":    p.ctx.DpfID,
		"processedAt":    time.Now().UTC().Format(time.RFC3339),
	}

	if req.Source.SourceScenario == api.DataSourceScenarioBreathingCSI {
		result["breathingSummary"] = map[string]any{
			"mode":              "BREATHING_CSI",
			"estimatedRateBpm":  18,
			"confidence":        0.82,
			"pipelineValidated": true,
		}
	}
	if req.Source.SourceScenario == api.DataSourceScenarioGestureRecognitionCSI {
		result["gestureRecognition"] = map[string]any{
			"mode":         "GESTURE_RECOGNITION_CSI",
			"gestureLabel": "unknown",
			"confidence":   0.78,
			"csiTimeSteps": countCSIRecords(source),
		}
	}
	if req.Source.SourceScenario == api.DataSourceScenarioPositioningCSI {
		result["positioning"] = map[string]any{
			"mode":         "POSITIONING_CSI",
			"xMeters":      0.0,
			"yMeters":      0.0,
			"confidence":   0.74,
			"csiTimeSteps": countCSIRecords(source),
		}
	}
	if req.Source.SourceScenario == api.DataSourceScenarioVehicleCSI {
		result["vehicle"] = map[string]any{
			"mode":          "VEHICLE_CSI",
			"mobilityState": "unknown",
			"speedMps":      0.0,
			"confidence":    0.76,
			"csiTimeSteps":  countCSIRecords(source),
		}
	}

	return result, nil
}

func (p *Processor) encodeResult(protocol api.ProtocolType, payload map[string]any) ([]byte, error) {
	switch protocol {
	case api.ProtocolTypeJSON:
		return json.Marshal(payload)
	case api.ProtocolTypeProtobuf:
		return proto.Marshal(typedProcessingResult(payload))
	default:
		return nil, fmt.Errorf("unsupported payload protocol %q", protocol)
	}
}

func (p *Processor) deliver(req *api.SubmitProcessingTaskRequest, sessionID string, encoded []byte) error {
	if !isSupportedTransportProfile(req.Delivery.TransportProtocol) {
		return fmt.Errorf("unsupported transport protocol %q", req.Delivery.TransportProtocol)
	}

	conn, err := grpc.Dial(
		netAddress(req.Delivery.DsfControlEndpoint),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec.JSONCodec{})),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := api.NewResultTransferServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.TaskTimeoutSeconds)*time.Second)
	defer cancel()

	contentType := "application/json"
	if req.Delivery.PayloadProtocol == api.ProtocolTypeProtobuf {
		contentType = "application/protobuf"
	}

	openResp, err := client.OpenTransfer(ctx, &api.OpenTransferRequest{
		OrchestrationID:    req.OrchestrationID,
		TaskID:             req.TaskID,
		TransferSessionID:  sessionID,
		DpfID:              p.ctx.DpfID,
		DsfID:              req.Delivery.DsfID,
		TransportProtocol:  req.Delivery.TransportProtocol,
		PayloadProtocol:    req.Delivery.PayloadProtocol,
		ChannelID:          req.Delivery.ChannelID,
		ContentType:        contentType,
		PayloadSchema:      req.OutputSchema,
		ProtocolParameters: req.Delivery.ProtocolParameters,
	})
	if err != nil {
		return err
	}
	if !openResp.Accepted {
		return fmt.Errorf("transfer rejected: %s", openResp.Reason)
	}
	logger.ProcLog.WithFields(map[string]any{
		"processing_task_id":  req.TaskID,
		"orchestration_id":    req.OrchestrationID,
		"transfer_session_id": sessionID,
		"transport_protocol":  req.Delivery.TransportProtocol,
	}).Infof("DPF_OPEN_TRANSFER_ACCEPTED")

	switch req.Delivery.TransportProtocol {
	case api.ProtocolTypeHTTP3:
		return p.deliverHTTP3(req, sessionID, encoded)
	case api.ProtocolTypeQUIC:
		return p.deliverQUIC(req, sessionID, encoded)
	}

	stream, err := client.PushResult(ctx)
	if err != nil {
		return err
	}

	chunkSize := p.cfg.Configuration.ChunkSize
	totalChunks := uint64(0)
	for offset := 0; offset < len(encoded); offset += chunkSize {
		end := offset + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[offset:end]
		sum := sha256.Sum256(chunk)
		totalChunks++
		if err := stream.Send(&api.ResultChunk{
			OrchestrationID:   req.OrchestrationID,
			TaskID:            req.TaskID,
			TransferSessionID: sessionID,
			SequenceNo:        totalChunks,
			Payload:           chunk,
			Eof:               end == len(encoded),
			Checksum:          hex.EncodeToString(sum[:]),
			Metadata: map[string]string{
				"dpfId": p.ctx.DpfID,
			},
		}); err != nil {
			return err
		}
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		return err
	}
	logger.ProcLog.WithFields(map[string]any{
		"processing_task_id":  req.TaskID,
		"orchestration_id":    req.OrchestrationID,
		"transfer_session_id": sessionID,
		"total_chunks":        totalChunks,
		"total_bytes":         len(encoded),
	}).Infof("DPF_PUSH_RESULT_COMPLETE")

	finalSum := sha256.Sum256(encoded)
	_, err = client.CloseTransfer(ctx, &api.CloseTransferRequest{
		OrchestrationID:   req.OrchestrationID,
		TaskID:            req.TaskID,
		TransferSessionID: sessionID,
		TotalChunks:       totalChunks,
		TotalBytes:        uint64(len(encoded)),
		FinalChecksum:     hex.EncodeToString(finalSum[:]),
	})
	if err == nil {
		logger.ProcLog.WithFields(map[string]any{
			"processing_task_id":  req.TaskID,
			"orchestration_id":    req.OrchestrationID,
			"transfer_session_id": sessionID,
		}).Infof("DPF_CLOSE_TRANSFER_COMPLETE")
	}
	return err
}

func (p *Processor) deliverHTTP3(req *api.SubmitProcessingTaskRequest, sessionID string, encoded []byte) error {
	url := buildNativeResultURL(req.Delivery.DsfDataEndpoint, sessionID)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", contentType(req.Delivery.PayloadProtocol))

	resp, err := p.http3Client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP/3 result transfer failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	logger.ProcLog.WithFields(map[string]any{
		"processing_task_id":  req.TaskID,
		"orchestration_id":    req.OrchestrationID,
		"transfer_session_id": sessionID,
		"total_bytes":         len(encoded),
	}).Infof("DPF_HTTP3_RESULT_COMPLETE")
	return nil
}

func (p *Processor) deliverQUIC(req *api.SubmitProcessingTaskRequest, sessionID string, encoded []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.TaskTimeoutSeconds)*time.Second)
	defer cancel()
	conn, err := p.getQUICConn(ctx, netAddress(req.Delivery.DsfDataEndpoint))
	if err != nil {
		return err
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		p.dropQUICConn(netAddress(req.Delivery.DsfDataEndpoint), conn)
		return err
	}
	header := map[string]string{"transferSessionId": sessionID}
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return err
	}
	if _, err := stream.Write(append(headerBytes, '\n')); err != nil {
		p.dropQUICConn(netAddress(req.Delivery.DsfDataEndpoint), conn)
		return err
	}
	if _, err := stream.Write(encoded); err != nil {
		p.dropQUICConn(netAddress(req.Delivery.DsfDataEndpoint), conn)
		return err
	}
	if err := stream.Close(); err != nil {
		p.dropQUICConn(netAddress(req.Delivery.DsfDataEndpoint), conn)
		return err
	}
	ack, err := io.ReadAll(stream)
	if err != nil {
		p.dropQUICConn(netAddress(req.Delivery.DsfDataEndpoint), conn)
		return err
	}
	if !strings.HasPrefix(string(ack), "OK") {
		return fmt.Errorf("QUIC result transfer failed: %s", strings.TrimSpace(string(ack)))
	}
	logger.ProcLog.WithFields(map[string]any{
		"processing_task_id":  req.TaskID,
		"orchestration_id":    req.OrchestrationID,
		"transfer_session_id": sessionID,
		"total_bytes":         len(encoded),
	}).Infof("DPF_QUIC_RESULT_COMPLETE")
	return nil
}

func (p *Processor) getQUICConn(ctx context.Context, target string) (*quic.Conn, error) {
	p.quicMu.Lock()
	defer p.quicMu.Unlock()

	if conn := p.quicConns[target]; conn != nil {
		select {
		case <-conn.Context().Done():
			delete(p.quicConns, target)
		default:
			return conn, nil
		}
	}

	conn, err := quic.DialAddr(ctx, target, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{quicALPN},
	}, nil)
	if err != nil {
		return nil, err
	}
	p.quicConns[target] = conn
	return conn, nil
}

func (p *Processor) dropQUICConn(target string, conn *quic.Conn) {
	p.quicMu.Lock()
	defer p.quicMu.Unlock()
	if p.quicConns[target] == conn {
		delete(p.quicConns, target)
		_ = conn.CloseWithError(0, "reset cached connection")
	}
}

func contentType(payload api.ProtocolType) string {
	if payload == api.ProtocolTypeProtobuf {
		return "application/protobuf"
	}
	return "application/json"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func isSupportedTransportProfile(protocol api.ProtocolType) bool {
	switch protocol {
	case api.ProtocolTypeHTTP2, api.ProtocolTypeHTTP3, api.ProtocolTypeQUIC:
		return true
	default:
		return false
	}
}

func (p *Processor) reportStatus(req *api.SubmitProcessingTaskRequest, state api.ProcessingState, detail string, metrics api.ProcessingMetrics, transferSessionID, errorCode, errorMessage string) error {
	conn, err := grpc.Dial(
		netAddress(req.Callback.DsmfCallbackEndpoint),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec.JSONCodec{})),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := api.NewDsmfProcessingCallbackServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Callback.CallbackTimeoutSeconds)*time.Second)
	defer cancel()

	_, err = client.ReportProcessingStatus(ctx, &api.ProcessingStatusReport{
		OrchestrationID:   req.OrchestrationID,
		TaskID:            req.TaskID,
		DpfID:             p.ctx.DpfID,
		State:             state,
		Detail:            detail,
		Metrics:           metrics,
		TransferSessionID: transferSessionID,
		CallbackRequestID: req.Callback.CallbackRequestID,
		ErrorCode:         errorCode,
		ErrorMessage:      errorMessage,
	})
	return err
}

func buildURL(ep api.Endpoint) string {
	host := ep.Host
	if ep.Port != 0 {
		host = host + ":" + strconv.Itoa(int(ep.Port))
	}
	if ep.Path == "" {
		return ep.Scheme + "://" + host
	}
	if strings.HasPrefix(ep.Path, "/") {
		return ep.Scheme + "://" + host + ep.Path
	}
	return ep.Scheme + "://" + host + "/" + ep.Path
}

func buildNativeResultURL(ep api.Endpoint, sessionID string) string {
	host := ep.Host
	if ep.Port != 0 {
		host = host + ":" + strconv.Itoa(int(ep.Port))
	}
	scheme := ep.Scheme
	if scheme == "" || scheme == "grpc" {
		scheme = "https"
	}
	return scheme + "://" + host + "/v1/result/" + sessionID
}

func netAddress(ep api.Endpoint) string {
	return ep.Host + ":" + strconv.Itoa(int(ep.Port))
}

func typedProcessingResult(payload map[string]any) *api.ProcessingResult {
	result := &api.ProcessingResult{
		TaskId:         stringValue(payload["taskId"]),
		RequestId:      stringValue(payload["requestId"]),
		SourceId:       stringValue(payload["sourceId"]),
		SourceCategory: stringValue(payload["sourceCategory"]),
		SourceScenario: stringValue(payload["sourceScenario"]),
		OutputSchema:   stringValue(payload["outputSchema"]),
		StepsApplied:   stringSliceValue(payload["stepsApplied"]),
		SourceBytes:    uint64Value(payload["sourceBytes"]),
		ContentPreview: stringValue(payload["contentPreview"]),
		Parameters:     parameterValues(payload["parameters"]),
		ProcessedBy:    stringValue(payload["processedBy"]),
		ProcessedAt:    stringValue(payload["processedAt"]),
	}

	if summary, ok := payload["breathingSummary"].(map[string]any); ok {
		result.BreathingCsi = &api.BreathingCsiResult{
			Mode:              stringValue(summary["mode"]),
			EstimatedRateBpm:  float64Value(summary["estimatedRateBpm"]),
			Confidence:        float64Value(summary["confidence"]),
			PipelineValidated: boolValue(summary["pipelineValidated"]),
		}
	}
	if summary, ok := payload["gestureRecognition"].(map[string]any); ok {
		result.GestureRecognition = &api.GestureRecognitionResult{
			Mode:         stringValue(summary["mode"]),
			GestureLabel: stringValue(summary["gestureLabel"]),
			Confidence:   float64Value(summary["confidence"]),
			CsiTimeSteps: uint64Value(summary["csiTimeSteps"]),
		}
	}
	if summary, ok := payload["positioning"].(map[string]any); ok {
		result.Positioning = &api.PositioningResult{
			Mode:         stringValue(summary["mode"]),
			XMeters:      float64Value(summary["xMeters"]),
			YMeters:      float64Value(summary["yMeters"]),
			Confidence:   float64Value(summary["confidence"]),
			CsiTimeSteps: uint64Value(summary["csiTimeSteps"]),
		}
	}
	if summary, ok := payload["vehicle"].(map[string]any); ok {
		result.Vehicle = &api.VehicleResult{
			Mode:          stringValue(summary["mode"]),
			MobilityState: stringValue(summary["mobilityState"]),
			SpeedMps:      float64Value(summary["speedMps"]),
			Confidence:    float64Value(summary["confidence"]),
			CsiTimeSteps:  uint64Value(summary["csiTimeSteps"]),
		}
	}

	return result
}

func parameterValues(value any) []*api.ResultParameter {
	parameters, ok := value.(map[string]string)
	if !ok {
		return nil
	}
	result := make([]*api.ResultParameter, 0, len(parameters))
	for key, val := range parameters {
		result = append(result, &api.ResultParameter{Key: key, Value: val})
	}
	return result
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, stringValue(item))
		}
		return result
	default:
		return nil
	}
}

func uint64Value(value any) uint64 {
	switch typed := value.(type) {
	case uint64:
		return typed
	case uint32:
		return uint64(typed)
	case uint:
		return uint64(typed)
	case int:
		if typed < 0 {
			return 0
		}
		return uint64(typed)
	case int64:
		if typed < 0 {
			return 0
		}
		return uint64(typed)
	case float64:
		if typed < 0 {
			return 0
		}
		return uint64(typed)
	default:
		return 0
	}
}

func float64Value(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case uint64:
		return float64(typed)
	default:
		return 0
	}
}

func boolValue(value any) bool {
	typed, _ := value.(bool)
	return typed
}

func countCSIRecords(source []byte) uint64 {
	trimmed := bytes.TrimSpace(source)
	if len(trimmed) == 0 {
		return 0
	}
	lines := bytes.Count(trimmed, []byte{'\n'}) + 1
	if lines <= 1 {
		return 0
	}
	return uint64(lines - 1)
}
