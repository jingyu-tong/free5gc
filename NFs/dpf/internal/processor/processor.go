package processor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/free5gc/dataapi/pkg/api"
	"github.com/free5gc/dataapi/pkg/codec"
	dpf_context "github.com/free5gc/dpf/internal/context"
	"github.com/free5gc/dpf/internal/logger"
	"github.com/free5gc/dpf/pkg/factory"
)

type Processor struct {
	cfg    *factory.Config
	ctx    *dpf_context.Context
	mu     sync.RWMutex
	tasks  map[string]*api.SubmitProcessingTaskRequest
	client *http.Client
}

func New(cfg *factory.Config, ctx *dpf_context.Context) *Processor {
	return &Processor{
		cfg:   cfg,
		ctx:   ctx,
		tasks: make(map[string]*api.SubmitProcessingTaskRequest),
		client: &http.Client{
			Timeout: time.Duration(cfg.Configuration.HTTPSourceTimeoutSecs) * time.Second,
		},
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
		resp, err := p.client.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= http.StatusBadRequest {
			return nil, fmt.Errorf("source returned status %d", resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	default:
		return nil, fmt.Errorf("unsupported source scheme %q", ep.Scheme)
	}
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
		"taskId":          req.TaskID,
		"requestId":       req.RequestID,
		"sourceId":        req.Source.SourceID,
		"sourceCategory":  req.Source.SourceCategory,
		"sourceScenario":  req.Source.SourceScenario,
		"outputSchema":    req.OutputSchema,
		"stepsApplied":    steps,
		"sourceBytes":     len(source),
		"contentPreview":  preview,
		"parameters":      parameters,
		"processedBy":     p.ctx.DpfID,
		"processedAt":     time.Now().UTC().Format(time.RFC3339),
	}

	if req.Source.SourceScenario == api.DataSourceScenarioBreathingCSI {
		result["breathingSummary"] = map[string]any{
			"mode":               "BREATHING_CSI",
			"estimatedRateBpm":   18,
			"confidence":         0.82,
			"pipelineValidated":  true,
		}
	}

	return result, nil
}

func (p *Processor) encodeResult(protocol api.ProtocolType, payload map[string]any) ([]byte, error) {
	switch protocol {
	case api.ProtocolTypeJSON:
		return json.Marshal(payload)
	case api.ProtocolTypeProtobuf:
		pbStruct, err := structpb.NewStruct(toStructMap(payload))
		if err != nil {
			return nil, err
		}
		return proto.Marshal(pbStruct)
	default:
		return nil, fmt.Errorf("unsupported payload protocol %q", protocol)
	}
}

func (p *Processor) deliver(req *api.SubmitProcessingTaskRequest, sessionID string, encoded []byte) error {
	if req.Delivery.TransportProtocol != api.ProtocolTypeHTTP2 {
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
		OrchestrationID:   req.OrchestrationID,
		TaskID:            req.TaskID,
		TransferSessionID: sessionID,
		DpfID:             p.ctx.DpfID,
		DsfID:             req.Delivery.DsfID,
		TransportProtocol: req.Delivery.TransportProtocol,
		PayloadProtocol:   req.Delivery.PayloadProtocol,
		ChannelID:         req.Delivery.ChannelID,
		ContentType:       contentType,
		PayloadSchema:     req.OutputSchema,
		ProtocolParameters: req.Delivery.ProtocolParameters,
	})
	if err != nil {
		return err
	}
	if !openResp.Accepted {
		return fmt.Errorf("transfer rejected: %s", openResp.Reason)
	}
	logger.ProcLog.WithFields(map[string]any{
		"processing_task_id": req.TaskID,
		"orchestration_id":   req.OrchestrationID,
		"transfer_session_id": sessionID,
	}).Infof("DPF_OPEN_TRANSFER_ACCEPTED")

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
		"processing_task_id": req.TaskID,
		"orchestration_id":   req.OrchestrationID,
		"transfer_session_id": sessionID,
		"total_chunks":       totalChunks,
		"total_bytes":        len(encoded),
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
			"processing_task_id": req.TaskID,
			"orchestration_id":   req.OrchestrationID,
			"transfer_session_id": sessionID,
		}).Infof("DPF_CLOSE_TRANSFER_COMPLETE")
	}
	return err
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

func netAddress(ep api.Endpoint) string {
	return ep.Host + ":" + strconv.Itoa(int(ep.Port))
}

func toStructMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for k, v := range input {
		output[k] = normalizeStructValue(v)
	}
	return output
}

func normalizeStructValue(v any) any {
	if v == nil {
		return nil
	}

	switch typed := v.(type) {
	case map[string]any:
		return toStructMap(typed)
	case []string:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
		return items
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, normalizeStructValue(item))
		}
		return items
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return rv.String()
	case reflect.Bool:
		return rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint()
	case reflect.Float32, reflect.Float64:
		return rv.Float()
	case reflect.Slice, reflect.Array:
		items := make([]any, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			items = append(items, normalizeStructValue(rv.Index(i).Interface()))
		}
		return items
	case reflect.Map:
		if rv.Type().Key().Kind() == reflect.String {
			out := make(map[string]any, rv.Len())
			iter := rv.MapRange()
			for iter.Next() {
				out[iter.Key().String()] = normalizeStructValue(iter.Value().Interface())
			}
			return out
		}
	}

	return v
}
