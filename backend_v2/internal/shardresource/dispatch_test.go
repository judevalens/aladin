package shardresource

import (
	"context"
	"encoding/json"
	"testing"
)

type dispatchRecorder struct {
	method   string
	mutation Mutation
}

func (r *dispatchRecorder) Hello(context.Context, Target) (map[string]any, error) {
	r.method = "hello"
	return map[string]any{"protocol": "bridge/2"}, nil
}
func (r *dispatchRecorder) Describe(context.Context, Target, ResourceRequest) (Descriptor, error) {
	r.method = "describe"
	return Descriptor{}, nil
}
func (r *dispatchRecorder) Read(context.Context, Target, ResourceRequest) (Snapshot, error) {
	r.method = "read"
	return Snapshot{}, nil
}
func (r *dispatchRecorder) Mutate(_ context.Context, _ Target, mutation Mutation) (MutationResult, error) {
	r.method, r.mutation = "mutate", mutation
	return MutationResult{RequestID: mutation.RequestID}, nil
}
func (*dispatchRecorder) Subscribe(context.Context, Target, ResourceRequest) (Subscription, error) {
	return Subscription{}, nil
}

func TestDispatchOwnsCommandMapping(t *testing.T) {
	params, _ := json.Marshal(map[string]any{"binding": "tasks", "requestId": "request-1", "data": map[string]any{"title": "x"}})
	recorder := &dispatchRecorder{}
	result, err := Dispatch(context.Background(), recorder, Target{}, BridgeCommand{Method: "resource.insert", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if recorder.method != "mutate" || recorder.mutation.Op != "insert" || result.(MutationResult).RequestID != "request-1" {
		t.Fatalf("unexpected dispatch: method=%q mutation=%+v result=%+v", recorder.method, recorder.mutation, result)
	}
}

func TestParseBridgeUsesProtocolSchema(t *testing.T) {
	valid := []byte(`{"aladin":"bridge/2","type":"request","id":7,"method":"hello","params":{}}`)
	command, err := ParseBridge(valid)
	if err != nil || command.ID != 7 || command.Method != "hello" {
		t.Fatalf("valid bridge request rejected: command=%+v err=%v", command, err)
	}
	if _, err := ParseBridge([]byte(`{"id":7,"method":"hello","params":{}}`)); ErrorCode(err, "") != "bad-request" {
		t.Fatalf("invalid bridge request did not fail closed: %v", err)
	}
}
