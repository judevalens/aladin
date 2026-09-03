package compat

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"aladin/backend_v2/internal/shardresource"
)

func TestLogV1ObserverEmitsOneMarkerPerOperationAndEnvironment(t *testing.T) {
	var output bytes.Buffer
	observer := NewLogV1Observer(slog.New(slog.NewTextHandler(&output, nil)))
	observer.Used("get", shardresource.EnvironmentPublished)
	observer.Used("get", shardresource.EnvironmentPublished)
	observer.Used("get", shardresource.EnvironmentDraft)
	if count := strings.Count(output.String(), "shard v1 compatibility used"); count != 2 {
		t.Fatalf("expected two distinct telemetry markers, got %d: %s", count, output.String())
	}
	if !strings.Contains(output.String(), V1RetirementCondition) {
		t.Fatalf("telemetry omitted retirement condition: %s", output.String())
	}
}
