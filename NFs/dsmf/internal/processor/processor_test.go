package processor

import (
	"context"
	"testing"
	"time"

	"github.com/free5gc/dataapi/pkg/api"
	dsmf_context "github.com/free5gc/dsmf/internal/context"
	"github.com/free5gc/dsmf/pkg/factory"
)

func TestCreateTaskRejectsUnsupportedTransport(t *testing.T) {
	cfg := &factory.Config{
		Configuration: factory.Configuration{
			DefaultProtocols: factory.ProtocolConfig{
				Transport: "HTTP2",
				Payload:   "JSON",
			},
			DpfEndpoints: []factory.EndpointRef{{ID: "dpf-1", Address: "127.0.0.1:50071"}},
			DsfEndpoints: []factory.EndpointRef{{ID: "dsf-1", Address: "127.0.0.1:50072"}},
			Task: factory.TaskConfig{TimeoutSeconds: 1},
		},
	}
	ctx := dsmf_context.New(cfg)
	proc := New(cfg, ctx)

	_, status, err := proc.CreateTask(Request{
		ResultMode:        "async",
		TransportProtocol: api.ProtocolTypeQUIC,
		PayloadProtocol:   api.ProtocolTypeJSON,
	})
	if err == nil {
		t.Fatalf("expected unsupported transport error")
	}
	if status != 400 {
		t.Fatalf("expected 400, got %d", status)
	}
}

func TestReportStorageStatusCompletesTask(t *testing.T) {
	cfg := &factory.Config{}
	ctx := &dsmf_context.Context{}
	proc := New(cfg, ctx)

	record := &taskRecord{
		view: TaskView{
			TaskID:          "task-1",
			StorageTaskID:   "store-1",
			UpdatedAt:       time.Now().UTC(),
		},
		done: make(chan struct{}),
	}
	proc.tasks["task-1"] = record
	proc.storageIndex["store-1"] = "task-1"

	ack, err := proc.ReportStorageStatus(context.Background(), &api.StorageStatusReport{
		TaskID:         "store-1",
		State:          api.StorageStateStored,
		ResultLocation: api.StorageResultLocation{ResultURI: "file:///tmp/result.json"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ack.Accepted {
		t.Fatalf("expected ack accepted")
	}

	view := proc.GetTask("task-1")
	if view.ResultURI == "" {
		t.Fatalf("expected result uri to be updated")
	}
	select {
	case <-record.done:
	default:
		t.Fatalf("expected task completion signal")
	}
}
