package processor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/free5gc/dataapi/pkg/api"
	"github.com/free5gc/dataapi/pkg/codec"
	dsf_context "github.com/free5gc/dsf/internal/context"
	"github.com/free5gc/dsf/internal/logger"
	"github.com/free5gc/dsf/pkg/factory"
)

type storageTask struct {
	request *api.SubmitStorageTaskRequest
}

type transferSession struct {
	taskID          string
	sessionID       string
	filePath        string
	file            *os.File
	buffer          []byte
	receivedChunks  uint64
	receivedBytes   uint64
	payloadProtocol api.ProtocolType
	transferState   api.TransferState
}

type Processor struct {
	cfg                *factory.Config
	ctx                *dsf_context.Context
	mu                 sync.RWMutex
	tasks              map[string]*storageTask
	orchestrationIndex map[string]string
	sessions           map[string]*transferSession
}

func New(cfg *factory.Config, ctx *dsf_context.Context) *Processor {
	return &Processor{
		cfg:                cfg,
		ctx:                ctx,
		tasks:              make(map[string]*storageTask),
		orchestrationIndex: make(map[string]string),
		sessions:           make(map[string]*transferSession),
	}
}

func (p *Processor) SubmitStorageTask(ctx context.Context, req *api.SubmitStorageTaskRequest) (*api.SubmitStorageTaskResponse, error) {
	p.mu.Lock()
	p.tasks[req.TaskID] = &storageTask{request: req}
	p.orchestrationIndex[req.OrchestrationID] = req.TaskID
	p.mu.Unlock()

	logger.StoreLog.WithFields(map[string]any{
		"storage_task_id":  req.TaskID,
		"orchestration_id": req.OrchestrationID,
		"request_id":       req.RequestID,
	}).Infof("DSF_SUBMIT_STORAGE_TASK")

	go p.reportStatus(req, api.StorageStateReadyToReceive, "ready to receive", api.StorageResultLocation{}, "", "")

	return &api.SubmitStorageTaskResponse{
		TaskID:   req.TaskID,
		Accepted: true,
		DsfID:    p.ctx.DsfID,
		State:    api.StorageStateAccepted,
	}, nil
}

func (p *Processor) OpenTransfer(ctx context.Context, req *api.OpenTransferRequest) (*api.OpenTransferResponse, error) {
	p.mu.RLock()
	task := p.tasks[req.TaskID]
	if task == nil {
		if storageTaskID, ok := p.orchestrationIndex[req.OrchestrationID]; ok {
			task = p.tasks[storageTaskID]
		}
	}
	p.mu.RUnlock()
	if task == nil {
		return &api.OpenTransferResponse{
			Accepted:          false,
			TransferSessionID: req.TransferSessionID,
			State:             api.TransferStateRejected,
			Reason:            "storage task not found",
		}, nil
	}
	if !isSupportedTransportProfile(req.TransportProtocol) {
		return &api.OpenTransferResponse{
			Accepted:          false,
			TransferSessionID: req.TransferSessionID,
			State:             api.TransferStateRejected,
			Reason:            "unsupported transport protocol",
		}, nil
	}
	if req.PayloadProtocol != api.ProtocolTypeJSON && req.PayloadProtocol != api.ProtocolTypeProtobuf {
		return &api.OpenTransferResponse{
			Accepted:          false,
			TransferSessionID: req.TransferSessionID,
			State:             api.TransferStateRejected,
			Reason:            "unsupported payload protocol",
		}, nil
	}

	dir := filepath.Join(p.ctx.RootDir, task.request.StoragePolicy.ObjectPrefix)
	if err := os.MkdirAll(dir, 0o775); err != nil {
		return nil, err
	}
	ext := ".bin"
	switch req.PayloadProtocol {
	case api.ProtocolTypeJSON:
		ext = ".json"
	case api.ProtocolTypeProtobuf:
		ext = ".pb"
	}
	filePath := filepath.Join(dir, req.TaskID+"-"+req.TransferSessionID+ext)
	if !task.request.StoragePolicy.OverwriteIfExists {
		if _, err := os.Stat(filePath); err == nil {
			return &api.OpenTransferResponse{
				Accepted:          false,
				TransferSessionID: req.TransferSessionID,
				State:             api.TransferStateRejected,
				Reason:            "target file already exists",
			}, nil
		}
	}
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.sessions[req.TransferSessionID] = &transferSession{
		taskID:          task.request.TaskID,
		sessionID:       req.TransferSessionID,
		filePath:        filePath,
		file:            file,
		payloadProtocol: req.PayloadProtocol,
		transferState:   api.TransferStateOpen,
	}
	p.mu.Unlock()

	logger.StoreLog.WithFields(map[string]any{
		"storage_task_id":     task.request.TaskID,
		"orchestration_id":    req.OrchestrationID,
		"transfer_session_id": req.TransferSessionID,
		"transport_protocol":  req.TransportProtocol,
		"payload_protocol":    req.PayloadProtocol,
	}).Infof("DSF_OPEN_TRANSFER_ACCEPTED")

	go p.reportStatus(task.request, api.StorageStateReceiving, "transfer session opened", api.StorageResultLocation{}, "", "")

	return &api.OpenTransferResponse{
		Accepted:          true,
		TransferSessionID: req.TransferSessionID,
		State:             api.TransferStateOpen,
	}, nil
}

func isSupportedTransportProfile(protocol api.ProtocolType) bool {
	switch protocol {
	case api.ProtocolTypeHTTP2, api.ProtocolTypeHTTP3, api.ProtocolTypeQUIC:
		return true
	default:
		return false
	}
}

func (p *Processor) StoreNativePayload(ctx context.Context, sessionID string, body io.Reader) (*api.CloseTransferResponse, error) {
	p.mu.RLock()
	session := p.sessions[sessionID]
	p.mu.RUnlock()
	if session == nil {
		return &api.CloseTransferResponse{
			TransferSessionID: sessionID,
			State:             api.TransferStateFailed,
			Message:           "transfer session not found",
		}, nil
	}

	buffer := make([]byte, 32*1024)
	for {
		n, err := body.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			if _, writeErr := session.file.Write(chunk); writeErr != nil {
				return nil, writeErr
			}
			session.buffer = append(session.buffer, chunk...)
			session.receivedChunks++
			session.receivedBytes += uint64(n)
			session.transferState = api.TransferStateReceiving
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	return p.closeSession(session, "", "")
}

func (p *Processor) PushResult(stream api.ResultTransferService_PushResultServer) error {
	var current *transferSession
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			if current == nil {
				return stream.SendAndClose(&api.PushResultResponse{
					State:  api.TransferStateFailed,
					Reason: "no chunks received",
				})
			}
			return stream.SendAndClose(&api.PushResultResponse{
				TransferSessionID: chunkOrEmpty(current),
				State:             api.TransferStateReceiving,
				ReceivedChunks:    current.receivedChunks,
				ReceivedBytes:     current.receivedBytes,
			})
		}
		if err != nil {
			return err
		}

		p.mu.RLock()
		current = p.sessions[chunk.TransferSessionID]
		p.mu.RUnlock()
		if current == nil {
			return fmt.Errorf("transfer session %s not found", chunk.TransferSessionID)
		}

		if _, err := current.file.Write(chunk.Payload); err != nil {
			return err
		}
		current.buffer = append(current.buffer, chunk.Payload...)
		current.receivedChunks++
		current.receivedBytes += uint64(len(chunk.Payload))
		current.transferState = api.TransferStateReceiving
	}
}

func (p *Processor) CloseTransfer(ctx context.Context, req *api.CloseTransferRequest) (*api.CloseTransferResponse, error) {
	p.mu.Lock()
	session := p.sessions[req.TransferSessionID]
	p.mu.Unlock()
	if session == nil {
		return &api.CloseTransferResponse{
			TransferSessionID: req.TransferSessionID,
			State:             api.TransferStateFailed,
			Message:           "transfer session not found",
		}, nil
	}

	return p.closeSession(session, req.OrchestrationID, req.TransferSessionID)
}

func (p *Processor) closeSession(session *transferSession, orchestrationID, transferSessionID string) (*api.CloseTransferResponse, error) {
	if transferSessionID == "" {
		transferSessionID = session.sessionID
	}
	sum := sha256.Sum256(session.buffer)
	if session.file != nil {
		if err := session.file.Close(); err != nil {
			return nil, err
		}
		session.file = nil
	}
	session.transferState = api.TransferStateCompleted

	p.mu.RLock()
	task := p.tasks[session.taskID]
	p.mu.RUnlock()
	if task != nil {
		if orchestrationID == "" {
			orchestrationID = task.request.OrchestrationID
		}
		location := api.StorageResultLocation{
			ResultURI:   "file://" + session.filePath,
			ResultID:    uuid.NewString(),
			Checksum:    hex.EncodeToString(sum[:]),
			StoredBytes: session.receivedBytes,
		}
		logger.StoreLog.WithFields(map[string]any{
			"storage_task_id":     task.request.TaskID,
			"orchestration_id":    orchestrationID,
			"transfer_session_id": transferSessionID,
			"stored_bytes":        session.receivedBytes,
			"result_uri":          location.ResultURI,
		}).Infof("DSF_CLOSE_TRANSFER_COMPLETE")
		go p.reportStatus(task.request, api.StorageStateStored, "result stored", location, "", "")
	}

	return &api.CloseTransferResponse{
		TransferSessionID: transferSessionID,
		State:             api.TransferStateCompleted,
		StorageTicket:     transferSessionID + ":" + strconv.FormatUint(session.receivedBytes, 10),
		Message:           "stored successfully",
	}, nil
}

func (p *Processor) reportStatus(req *api.SubmitStorageTaskRequest, state api.StorageState, detail string, location api.StorageResultLocation, errorCode, errorMessage string) {
	conn, err := grpc.Dial(
		req.Callback.DsmfCallbackEndpoint.Host+":"+strconv.Itoa(int(req.Callback.DsmfCallbackEndpoint.Port)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec.JSONCodec{})),
	)
	if err != nil {
		logger.StoreLog.Warnf("failed to dial DSMF callback: %v", err)
		return
	}
	defer conn.Close()

	client := api.NewDsmfStorageCallbackServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Callback.CallbackTimeoutSeconds)*time.Second)
	defer cancel()

	_, err = client.ReportStorageStatus(ctx, &api.StorageStatusReport{
		OrchestrationID:   req.OrchestrationID,
		TaskID:            req.TaskID,
		DsfID:             p.ctx.DsfID,
		State:             state,
		Detail:            detail,
		ResultLocation:    location,
		CallbackRequestID: req.Callback.CallbackRequestID,
		ErrorCode:         errorCode,
		ErrorMessage:      errorMessage,
	})
	if err != nil {
		logger.StoreLog.Warnf("failed to report storage status: %v", err)
		return
	}
	logger.StoreLog.WithFields(map[string]any{
		"storage_task_id":  req.TaskID,
		"orchestration_id": req.OrchestrationID,
		"state":            state,
		"detail":           detail,
		"result_uri":       location.ResultURI,
	}).Infof("DSF_REPORT_STORAGE_STATUS")
}

func chunkOrEmpty(session *transferSession) string {
	if session == nil {
		return ""
	}
	return session.sessionID
}
