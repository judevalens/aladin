package docsurface

import (
	"encoding/json"
	"testing"

	"aladin/backend_v2/internal/shardv2"
)

func TestV2ManifestBindingsAreOptIn(t *testing.T) {
	manifest := Manifest{Version: 1, Anchors: []ManifestAnchor{{ID: "items", Route: "/", Meaning: "Items", Binding: json.RawMessage(`{"evaluation":"legacy provenance"}`)}}}
	if errors := ValidateManifestBindingsV2(manifest, nil); len(errors) != 0 {
		t.Fatalf("v1 provenance must remain opaque: %v", errors)
	}
	compiled := &shardv2.Compiled{Contract: shardv2.Contract{Bindings: map[string]shardv2.Binding{"itemsView": {Resource: "items"}}}}
	if errors := ValidateManifestBindingsV2(manifest, compiled); len(errors) == 0 {
		t.Fatal("v2 accepted legacy opaque provenance as a binding")
	}
	manifest.Anchors[0].Binding = json.RawMessage(`{"id":"missing"}`)
	if errors := ValidateManifestBindingsV2(manifest, compiled); len(errors) == 0 {
		t.Fatal("v2 accepted an unknown binding")
	}
	manifest.Anchors[0].Binding = json.RawMessage(`{"id":"itemsView"}`)
	if errors := ValidateManifestBindingsV2(manifest, compiled); len(errors) != 0 {
		t.Fatalf("known v2 binding rejected: %v", errors)
	}
}
