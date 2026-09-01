package docsurface

import (
	"strings"

	"aladin/backend_v2/internal/shardv2"
)

// CSPForBridgeVersion is selected from protected release metadata, never an
// iframe parameter or mutable authoring file. V2 allows AJV runtime compilation,
// explicitly authorized by the user. Existing v1 policy remains unchanged.
// Apply this after optional vendor-origin augmentation in protected serving.
func CSPForBridgeVersion(policy, bridgeVersion string) string {
	if bridgeVersion != shardv2.BridgeVersion {
		return policy
	}
	return strings.Replace(policy, "script-src 'unsafe-inline'", "script-src 'unsafe-inline' 'unsafe-eval'", 1)
}
