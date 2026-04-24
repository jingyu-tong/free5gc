package processor

import (
	"testing"

	"github.com/free5gc/dataapi/pkg/api"
	dpf_context "github.com/free5gc/dpf/internal/context"
	"github.com/free5gc/dpf/pkg/factory"
)

func TestEncodeResultSupportsJSONAndProtobuf(t *testing.T) {
	cfg := &factory.Config{
		Configuration: factory.Configuration{
			HTTPSourceTimeoutSecs: 1,
		},
	}
	ctx := &dpf_context.Context{DpfID: "dpf-1"}
	proc := New(cfg, ctx)

	payload := map[string]any{"hello": "world"}

	jsonBytes, err := proc.encodeResult(api.ProtocolTypeJSON, payload)
	if err != nil {
		t.Fatalf("unexpected JSON error: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Fatalf("expected JSON bytes")
	}

	protoBytes, err := proc.encodeResult(api.ProtocolTypeProtobuf, payload)
	if err != nil {
		t.Fatalf("unexpected protobuf error: %v", err)
	}
	if len(protoBytes) == 0 {
		t.Fatalf("expected protobuf bytes")
	}
}
