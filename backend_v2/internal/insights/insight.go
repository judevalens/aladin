package insights

import "encoding/json"

type InsightType string

const (
	TypeBridge       InsightType = "bridge"
	TypeConvergence  InsightType = "convergence"
	TypeTrend        InsightType = "trend"
	TypeContradiction InsightType = "contradiction"
)

type Insight struct {
	Type        InsightType
	Title       string
	Body        string
	Entity      string
	Topic       string
	ArtifactIDs []string
	Confidence  float64
}

func (i Insight) Key() string {
	if i.Entity != "" {
		return i.Entity
	}
	return i.Topic
}

func (i Insight) ArtifactIDsJSON() string {
	b, _ := json.Marshal(i.ArtifactIDs)
	return string(b)
}
