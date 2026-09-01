package shardv2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const sharedRoot = "../../../shared/shard-v2"

func fixturesPath(t *testing.T, name string) string {
	t.Helper()
	// Package cwd is backend_v2/internal/shardv2.
	return filepath.Join(sharedRoot, name)
}

func TestSharedValidationFixtures(t *testing.T) {
	data, err := os.ReadFile(fixturesPath(t, "fixtures/validation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Providers Registry
		Cases     []struct {
			Name, Kind string
			Valid      bool
			Value      json.RawMessage
			Schema     Schema
			Resource   Resource
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, tc := range corpus.Cases {
		t.Run(tc.Kind+"/"+tc.Name, func(t *testing.T) {
			value, err := DecodeJSON(tc.Value)
			if err == nil {
				switch tc.Kind {
				case "contract":
					_, err = Compile(tc.Value, corpus.Providers)
				case "schema":
					err = ValidateSchema(value.(map[string]any))
				case "data":
					err = ValidateData(tc.Schema, value)
				case "query":
					err = ValidateProtocol("query", value)
					if err == nil {
						var query Query
						err = json.Unmarshal(tc.Value, &query)
						if err == nil {
							err = ValidateQuery(tc.Resource, query)
						}
					}
				case "event":
					_, err = ValidateEvent(tc.Value, tc.Resource, tc.Resource.Schema)
				default:
					err = ValidateProtocol(tc.Kind, value)
				}
			}
			if (err == nil) != tc.Valid {
				t.Fatalf("want valid=%v, got %v", tc.Valid, err)
			}
		})
	}
}

func TestGeneratedSchemaAndFixtureHashes(t *testing.T) {
	raw, err := os.ReadFile(fixturesPath(t, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string
		Files   map[string]string
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.0.0" {
		t.Fatal("unexpected fixture version")
	}
	for path, expected := range manifest.Files {
		data, err := os.ReadFile(fixturesPath(t, path))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != expected {
			t.Errorf("%s: regenerate shared manifest", path)
		}
	}
	for name, embedded := range protocolSchemas {
		data, err := os.ReadFile(fixturesPath(t, "schemas/"+name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var a, b any
		if err := json.Unmarshal(data, &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(embedded), &b); err != nil {
			t.Fatal(err)
		}
		ca, _ := json.Marshal(a)
		cb, _ := json.Marshal(b)
		if string(ca) != string(cb) {
			t.Errorf("%s: generated Go schema drift", name)
		}
	}
}

func TestProjectionPreservesRequiredAndNestedPrivacy(t *testing.T) {
	raw := []byte(`{"type":"object","properties":{"profile":{"type":"object","properties":{"name":{"type":"string"},"secret":{"type":"string"}},"required":["name","secret"]}},"required":["profile"]}`)
	var schema Schema
	_ = json.Unmarshal(raw, &schema)
	projected, err := ProjectSchema(schema, []string{"/profile/name"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		data  string
		valid bool
	}{
		{`{"profile":{"name":"Ada"}}`, true},
		{`{"profile":{"name":"Ada","secret":"hidden"}}`, false},
		{`{"profile":{}}`, false},
		{`{}`, false},
	} {
		value, _ := DecodeJSON([]byte(tc.data))
		if err := ValidateData(projected, value); (err == nil) != tc.valid {
			t.Fatalf("%s: %v", tc.data, err)
		}
	}
}

func TestExposureNeedsCurrentAuthorization(t *testing.T) {
	r := Resource{Exposure: Exposure{App: []string{"snapshot", "update"}, Agent: []string{"snapshot"}}}
	if got := EffectiveCapabilities(r, "agent", []string{"snapshot", "update"}); len(got) != 1 || got[0] != "snapshot" {
		t.Fatal(got)
	}
	if got := EffectiveCapabilities(r, "app", nil); len(got) != 0 {
		t.Fatal(got)
	}
	if got := EffectiveCapabilities(r, "invented", []string{"snapshot"}); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestTypedQueryOperandsUseJSONDomain(t *testing.T) {
	resource := Resource{Operations: []string{"snapshot", "query"}, Schema: Schema{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}, "count": map[string]any{"type": "number"}}}, Query: QueryPolicy{FilterFields: []string{"/title", "/count"}}}
	for _, predicate := range []Predicate{{Field: "/title", Op: "in", Value: []string{"a", "b"}}, {Field: "/count", Op: "gt", Value: 2}} {
		if err := ValidateQuery(resource, Query{Where: &predicate}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEmptySnapshotRoundTrip(t *testing.T) {
	event := Event{Protocol: StreamVersion, SubscriptionID: "s1", Resource: "shard://shard1/resources/items?view=v1", Epoch: "e1", Seq: "0", Op: "snapshot", Complete: true}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ValidateEvent(raw, Resource{Kind: "collection", SchemaVersion: 1}, Schema{"type": "object"})
	if err != nil {
		t.Fatalf("invalid serialized empty snapshot: %s: %v", raw, err)
	}
	if parsed.Records == nil || len(parsed.Records) != 0 || !parsed.Complete {
		t.Fatal("empty snapshot changed meaning")
	}
}
