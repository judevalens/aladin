package workers

import "encoding/json"

// stagePayload is the envelope for all pipeline stage tasks.
type stagePayload struct {
	ArtifactID string `json:"artifact_id"`
}

// parseArtifactID extracts the artifact ID from an asynq task payload.
// Falls back to treating the raw bytes as the ID for backwards compatibility.
func parseArtifactID(payload []byte) string {
	var p stagePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return string(payload)
	}
	return p.ArtifactID
}
