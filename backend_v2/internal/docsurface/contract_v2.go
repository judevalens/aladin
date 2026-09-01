package docsurface

import (
	"fmt"

	"aladin/backend_v2/internal/shardv2"
)

// CompileContractV2 is opt-in. V1 manifest validation and build behavior are
// unchanged; v2 serving is selected only from protected release metadata.
func CompileContractV2(data []byte, providers shardv2.Registry) (*shardv2.Compiled, error) {
	return shardv2.Compile(data, providers)
}

// ValidateManifestBindingsV2 adds semantic binding checks only when the caller
// explicitly supplies a compiled v2 contract. Legacy provenance stays opaque.
func ValidateManifestBindingsV2(manifest Manifest, compiled *shardv2.Compiled) []string {
	problems := ValidateManifest(manifest)
	if compiled == nil {
		return problems
	}
	for _, anchor := range manifest.Anchors {
		if len(anchor.Binding) == 0 {
			continue
		}
		raw, err := shardv2.DecodeJSON(anchor.Binding)
		if err != nil {
			problems = append(problems, fmt.Sprintf("anchor %q: invalid binding JSON", anchor.ID))
			continue
		}
		binding, ok := raw.(map[string]any)
		if !ok || len(binding) != 1 {
			problems = append(problems, fmt.Sprintf("anchor %q: v2 binding must contain only id", anchor.ID))
			continue
		}
		id, ok := binding["id"].(string)
		if !ok {
			problems = append(problems, fmt.Sprintf("anchor %q: binding id is required", anchor.ID))
			continue
		}
		if _, ok := compiled.Contract.Bindings[id]; !ok {
			problems = append(problems, fmt.Sprintf("anchor %q: unknown binding %q", anchor.ID, id))
		}
	}
	return problems
}
