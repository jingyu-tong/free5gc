package processor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/free5gc/dataapi/pkg/api"
	"github.com/free5gc/dataapi/pkg/codec"
	dsmf_context "github.com/free5gc/dsmf/internal/context"
	"github.com/free5gc/dsmf/internal/logger"
	"github.com/free5gc/dsmf/pkg/factory"
)

type Request struct {
	RequestID         string              `json:"requestId"`
	ResultMode        string              `json:"resultMode"`
	TransportProtocol api.ProtocolType    `json:"transportProtocol"`
	PayloadProtocol   api.ProtocolType    `json:"payloadProtocol"`
	DataSource        api.DataSourceSpec  `json:"dataSource"`
	ProcessingSteps   []api.ProcessingStep `json:"processingSteps"`
	OutputSchema      string              `json:"outputSchema"`
	TaskTimeoutSeconds uint32             `json:"taskTimeoutSeconds"`
	Labels            map[string]string   `json:"labels"`
}

type TaskView struct {
	TaskID           string               `json:"taskId"`
	RequestID        string               `json:"requestId"`
	OrchestrationID  string               `json:"orchestrationId"`
	ProcessingTaskID string               `json:"processingTaskId"`
	StorageTaskID    string               `json:"storageTaskId"`
	State            string               `json:"state"`
	SyncOrAsync      string               `json:"syncOrAsync"`
	ResultURI        string               `json:"resultUri,omitempty"`
	Error            string               `json:"error,omitempty"`
	ProcessingState  api.ProcessingState  `json:"processingState,omitempty"`
	StorageState     api.StorageState     `json:"storageState,omitempty"`
	UpdatedAt        time.Time            `json:"updatedAt"`
}

type taskRecord struct {
	view TaskView
	done chan struct{}
	once sync.Once
}

type Processor struct {
	cfg             *factory.Config
	ctx             *dsmf_context.Context
	mu              sync.RWMutex
	tasks           map[string]*taskRecord
	processingIndex map[string]string
	storageIndex    map[string]string
}

func New(cfg *factory.Config, ctx *dsmf_context.Context) *Processor {
	return &Processor{
		cfg:             cfg,
		ctx:             ctx,
		tasks:           make(map[string]*taskRecord),
		processingIndex: make(map[string]string),
		storageIndex:    make(map[string]string),
	}
}

func (p *Processor) CreateTask(req Request) (*TaskView, int, error) {
	transport := req.TransportProtocol
	if transport == "" {
		transport = api.ProtocolType(strings.ToUpper(p.cfg.Configuration.DefaultProtocols.Transport))
	}
	payload := req.PayloadProtocol
	if payload == "" {
		payload = api.ProtocolType(strings.ToUpper(p.cfg.Configuration.DefaultProtocols.Payload))
	}
	if transport != api.ProtocolTypeHTTP2 {
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported transport protocol %q", transport)
	}
	if payload != api.ProtocolTypeJSON && payload != api.ProtocolTypeProtobuf {
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported payload protocol %q", payload)
	}
	if len(p.cfg.Configuration.DpfEndpoints) == 0 || len(p.cfg.Configuration.DsfEndpoints) == 0 {
		return nil, http.StatusBadGateway, fmt.Errorf("DPF/DSF endpoint configuration is empty")
	}
	if req.ResultMode == "" {
		req.ResultMode = "sync"
	}
	if req.TaskTimeoutSeconds == 0 {
		req.TaskTimeoutSeconds = uint32(p.cfg.Configuration.Task.TimeoutSeconds)
	}
	if req.RequestID == "" {
		req.RequestID = uuid.NewString()
	}

	taskID := uuid.NewString()
	orchestrationID := uuid.NewString()
	processingTaskID := "proc-" + uuid.NewString()
	storageTaskID := "store-" + uuid.NewString()

	record := &taskRecord{
		view: TaskView{
			TaskID:           taskID,
			RequestID:        req.RequestID,
			OrchestrationID:  orchestrationID,
			ProcessingTaskID: processingTaskID,
			StorageTaskID:    storageTaskID,
			State:            "ACCEPTED",
			SyncOrAsync:      req.ResultMode,
			UpdatedAt:        time.Now().UTC(),
		},
		done: make(chan struct{}),
	}

	log := logger.ProcLog.WithFields(map[string]any{
		"task_id":           taskID,
		"request_id":        req.RequestID,
		"orchestration_id":  orchestrationID,
		"processing_task_id": processingTaskID,
		"storage_task_id":   storageTaskID,
		"result_mode":       req.ResultMode,
		"payload_protocol":  payload,
	})
	log.Infof("DSMF_CREATE_TASK_BEGIN")

	p.mu.Lock()
	p.tasks[taskID] = record
	p.processingIndex[processingTaskID] = taskID
	p.storageIndex[storageTaskID] = taskID
	p.mu.Unlock()

	dpfRef := p.cfg.Configuration.DpfEndpoints[0]
	dsfRef := p.cfg.Configuration.DsfEndpoints[0]
	dsfEndpoint, err := parseNetEndpoint(dsfRef.Address)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	callbackEndpoint, err := parseNetEndpoint(p.ctx.GRPCAddr)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	callback := api.CallbackBinding{
		DsmfCallbackEndpoint: callbackEndpoint,
		CallbackRequestID:    taskID,
		CallbackTimeoutSeconds: req.TaskTimeoutSeconds,
	}

	labels := make([]api.KvPair, 0, len(req.Labels))
	for k, v := range req.Labels {
		labels = append(labels, api.KvPair{Key: k, Value: v})
	}

	storageReq := &api.SubmitStorageTaskRequest{
		OrchestrationID: orchestrationID,
		TaskID:          storageTaskID,
		RequestID:       req.RequestID,
		ReceiveContract: api.ReceiveContract{
			DpfID:               dpfRef.ID,
			TransportProtocol:   transport,
			PayloadProtocol:     payload,
			ChannelID:           orchestrationID,
			ReceiveEndpoint:     dsfEndpoint,
			ExpectedContentType: contentType(payload),
		},
		StoragePolicy: api.StoragePolicy{
			BackendType:       p.cfg.Configuration.Task.Storage.BackendType,
			ObjectPrefix:      p.cfg.Configuration.Task.Storage.ObjectPrefix,
			RetentionDays:     p.cfg.Configuration.Task.Storage.RetentionDays,
			OverwriteIfExists: p.cfg.Configuration.Task.Storage.OverwriteIfExists,
		},
		ResultSchema: req.OutputSchema,
		Callback:     callback,
		Labels:       labels,
	}

	log.Infof("DSMF_SUBMIT_STORAGE_TASK_BEGIN")
	if err := p.submitStorageTask(dsfRef.Address, storageReq); err != nil {
		p.failTask(taskID, err.Error())
		return p.GetTask(taskID), http.StatusBadGateway, err
	}
	log.Infof("DSMF_SUBMIT_STORAGE_TASK_DONE")

	procReq := &api.SubmitProcessingTaskRequest{
		OrchestrationID: orchestrationID,
		TaskID:          processingTaskID,
		RequestID:       req.RequestID,
		Source:          req.DataSource,
		ProcessingSteps: req.ProcessingSteps,
		OutputSchema:    req.OutputSchema,
		Delivery: api.DeliveryBinding{
			DsfID:              dsfRef.ID,
			DsfControlEndpoint: dsfEndpoint,
			DsfDataEndpoint:    dsfEndpoint,
			TransportProtocol:  transport,
			PayloadProtocol:    payload,
			ChannelID:          orchestrationID,
		},
		Callback:          callback,
		TaskTimeoutSeconds: req.TaskTimeoutSeconds,
		Labels:            labels,
	}

	log.Infof("DSMF_SUBMIT_PROCESSING_TASK_BEGIN")
	if err := p.submitProcessingTask(dpfRef.Address, procReq); err != nil {
		p.failTask(taskID, err.Error())
		return p.GetTask(taskID), http.StatusBadGateway, err
	}
	log.Infof("DSMF_SUBMIT_PROCESSING_TASK_DONE")

	if req.ResultMode == "async" {
		log.Infof("DSMF_CREATE_TASK_ACCEPTED_ASYNC")
		return p.GetTask(taskID), http.StatusAccepted, nil
	}

	timeout := time.Duration(req.TaskTimeoutSeconds) * time.Second
	select {
	case <-record.done:
	case <-time.After(timeout):
		p.failTask(taskID, "task wait timeout")
		return p.GetTask(taskID), http.StatusGatewayTimeout, fmt.Errorf("task wait timeout")
	}

	view := p.GetTask(taskID)
	if view.Error != "" {
		log.WithField("error", view.Error).Warnf("DSMF_CREATE_TASK_FAILED")
		return view, http.StatusBadGateway, fmt.Errorf(view.Error)
	}
	log.WithField("result_uri", view.ResultURI).Infof("DSMF_CREATE_TASK_COMPLETE")
	return view, http.StatusOK, nil
}

func (p *Processor) GetTask(taskID string) *TaskView {
	p.mu.RLock()
	record := p.tasks[taskID]
	p.mu.RUnlock()
	if record == nil {
		return nil
	}
	view := record.view
	return &view
}

func (p *Processor) ReportProcessingStatus(_ context.Context, report *api.ProcessingStatusReport) (*api.ProcessingStatusAck, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	taskID := report.CallbackRequestID
	if taskID == "" {
		taskID = p.processingIndex[report.TaskID]
	}
	record := p.tasks[taskID]
	if record == nil {
		return &api.ProcessingStatusAck{Accepted: false, Message: "task not found"}, nil
	}

	record.view.ProcessingState = report.State
	record.view.UpdatedAt = time.Now().UTC()
	if report.ErrorMessage != "" {
		record.view.Error = report.ErrorMessage
	}
	logger.ProcLog.WithFields(map[string]any{
		"task_id":           taskID,
		"processing_task_id": report.TaskID,
		"orchestration_id":  report.OrchestrationID,
		"state":             report.State,
		"detail":            report.Detail,
		"transfer_session_id": report.TransferSessionID,
	}).Infof("DSMF_REPORT_PROCESSING_STATUS")
	p.refreshTaskState(record)
	return &api.ProcessingStatusAck{Accepted: true, Message: "accepted"}, nil
}

func (p *Processor) ReportStorageStatus(_ context.Context, report *api.StorageStatusReport) (*api.StorageStatusAck, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	taskID := report.CallbackRequestID
	if taskID == "" {
		taskID = p.storageIndex[report.TaskID]
	}
	record := p.tasks[taskID]
	if record == nil {
		return &api.StorageStatusAck{Accepted: false, Message: "task not found"}, nil
	}

	record.view.StorageState = report.State
	record.view.UpdatedAt = time.Now().UTC()
	if report.ResultLocation.ResultURI != "" {
		record.view.ResultURI = report.ResultLocation.ResultURI
	}
	if report.ErrorMessage != "" {
		record.view.Error = report.ErrorMessage
	}
	logger.ProcLog.WithFields(map[string]any{
		"task_id":          taskID,
		"storage_task_id":  report.TaskID,
		"orchestration_id": report.OrchestrationID,
		"state":            report.State,
		"detail":           report.Detail,
		"result_uri":       report.ResultLocation.ResultURI,
	}).Infof("DSMF_REPORT_STORAGE_STATUS")
	p.refreshTaskState(record)
	return &api.StorageStatusAck{Accepted: true, Message: "accepted"}, nil
}

func (p *Processor) failTask(taskID, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	record := p.tasks[taskID]
	if record == nil {
		return
	}
	record.view.State = "FAILED"
	record.view.Error = message
	record.view.UpdatedAt = time.Now().UTC()
	record.once.Do(func() { close(record.done) })
}

func (p *Processor) submitProcessingTask(target string, req *api.SubmitProcessingTaskRequest) error {
	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec.JSONCodec{})),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := api.NewDpfControlServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.TaskTimeoutSeconds)*time.Second)
	defer cancel()
	resp, err := client.SubmitProcessingTask(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Accepted {
		return fmt.Errorf("DPF rejected task: %s", resp.Reason)
	}
	return nil
}

func (p *Processor) submitStorageTask(target string, req *api.SubmitStorageTaskRequest) error {
	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec.JSONCodec{})),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := api.NewDsfControlServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Callback.CallbackTimeoutSeconds)*time.Second)
	defer cancel()
	resp, err := client.SubmitStorageTask(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Accepted {
		return fmt.Errorf("DSF rejected task: %s", resp.Reason)
	}
	return nil
}

func parseNetEndpoint(address string) (api.Endpoint, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return api.Endpoint{}, err
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil {
		return api.Endpoint{}, err
	}
	return api.Endpoint{
		Scheme: "grpc",
		Host:   host,
		Port:   uint32(parsedPort),
	}, nil
}

func contentType(payload api.ProtocolType) string {
	switch payload {
	case api.ProtocolTypeProtobuf:
		return "application/protobuf"
	default:
		return "application/json"
	}
}

func (p *Processor) refreshTaskState(record *taskRecord) {
	switch {
	case record.view.Error != "",
		record.view.ProcessingState == api.ProcessingStateFailed,
		record.view.StorageState == api.StorageStateFailed:
		record.view.State = "FAILED"
		record.once.Do(func() { close(record.done) })
	case record.view.ProcessingState == api.ProcessingStateCompleted &&
		record.view.StorageState == api.StorageStateStored:
		record.view.State = "COMPLETED"
		record.once.Do(func() { close(record.done) })
	case record.view.ProcessingState != "" &&
		record.view.ProcessingState != api.ProcessingStateCompleted:
		record.view.State = string(record.view.ProcessingState)
	case record.view.StorageState != "" &&
		record.view.StorageState != api.StorageStateStored:
		record.view.State = string(record.view.StorageState)
	case record.view.StorageState == api.StorageStateStored:
		record.view.State = string(record.view.StorageState)
	case record.view.ProcessingState != "":
		record.view.State = string(record.view.ProcessingState)
	default:
		record.view.State = "ACCEPTED"
	}
}
