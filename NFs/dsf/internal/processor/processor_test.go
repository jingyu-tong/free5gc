package processor

import (
	"context"
	"testing"

	"github.com/free5gc/dataapi/pkg/api"
	dsf_context "github.com/free5gc/dsf/internal/context"
	"github.com/free5gc/dsf/pkg/factory"
)

func TestOpenTransferRejectsUnknownTask(t *testing.T) {
	cfg := &factory.Config{}
	ctx := &dsf_context.Context{DsfID: "dsf-1", RootDir: t.TempDir()}
	proc := New(cfg, ctx)

	resp, err := proc.OpenTransfer(context.Background(), &api.OpenTransferRequest{
		TaskID:            "missing",
		TransferSessionID: "session-1",
		TransportProtocol: api.ProtocolTypeHTTP2,
		PayloadProtocol:   api.ProtocolTypeJSON,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Accepted {
		t.Fatalf("expected transfer to be rejected")
	}
}
