package processor

import (
	"testing"

	"github.com/free5gc/dataapi/pkg/api"
	dpf_context "github.com/free5gc/dpf/internal/context"
	"github.com/free5gc/dpf/pkg/factory"
	"google.golang.org/protobuf/proto"
)

func TestEncodeResultSupportsJSONAndProtobuf(t *testing.T) {
	cfg := &factory.Config{
		Configuration: factory.Configuration{
			HTTPSourceTimeoutSecs: 1,
		},
	}
	ctx := &dpf_context.Context{DpfID: "dpf-1"}
	proc := New(cfg, ctx)

	payload := map[string]any{
		"taskId":         "task-1",
		"requestId":      "request-1",
		"sourceId":       "source-1",
		"sourceCategory": "SENSING_CSI",
		"sourceScenario": "GESTURE_RECOGNITION_CSI",
		"outputSchema":   "gesture-result",
		"stepsApplied":   []string{"estimate-gesture"},
		"sourceBytes":    128,
		"contentPreview": "preview",
		"parameters":     map[string]string{"window": "1"},
		"processedBy":    "dpf-1",
		"processedAt":    "2026-04-28T00:00:00Z",
		"gestureRecognition": map[string]any{
			"mode":         "GESTURE_RECOGNITION_CSI",
			"gestureLabel": "unknown",
			"confidence":   0.78,
			"csiTimeSteps": uint64(1),
		},
	}

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
	var decoded api.ProcessingResult
	if err := proto.Unmarshal(protoBytes, &decoded); err != nil {
		t.Fatalf("protobuf bytes do not decode as ProcessingResult: %v", err)
	}
	if decoded.TaskId != "task-1" {
		t.Fatalf("unexpected decoded task id %q", decoded.TaskId)
	}
	if decoded.GestureRecognition == nil {
		t.Fatalf("expected typed gesture result")
	}
}
