// Package shardv2 owns the transport-neutral Shard v2 contract. It imports no
// service, repository, renderer, or HTTP package.
package shardv2

import "encoding/json"

const (
	ContractFile   = "contract.json"
	BridgeVersion  = "bridge/2"
	StreamVersion  = "shard-data/1"
	MaxJSONBytes   = 1 << 20
	MaxJSONDepth   = 64
	MaxRecordBytes = 64 << 10
	DefaultLimit   = 100
	MaxLimit       = 500
)

type Schema = map[string]any

type Contract struct {
	Version   int                 `json:"version"`
	Intent    string              `json:"intent"`
	Resources map[string]Resource `json:"resources"`
	Bindings  map[string]Binding  `json:"bindings"`
}
type Resource struct {
	URI           string       `json:"uri"`
	Kind          string       `json:"kind"`
	Meaning       string       `json:"meaning"`
	SchemaVersion int64        `json:"schemaVersion"`
	Schema        Schema       `json:"schema"`
	Source        Source       `json:"source"`
	Operations    []string     `json:"operations"`
	Observe       *Observation `json:"observe,omitempty"`
	Exposure      Exposure     `json:"exposure,omitempty"`
	Query         QueryPolicy  `json:"query,omitempty"`
}
type Source struct {
	Provider string         `json:"provider"`
	Version  int            `json:"version,omitempty"`
	Dataset  string         `json:"dataset,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}
type Observation struct {
	Mode     string `json:"mode"`
	Protocol string `json:"protocol"`
}
type Exposure struct {
	App   []string `json:"app,omitempty"`
	Agent []string `json:"agent,omitempty"`
}
type QueryPolicy struct {
	FilterFields []string `json:"filterFields,omitempty"`
	SortFields   []string `json:"sortFields,omitempty"`
	MaxLimit     int      `json:"maxLimit,omitempty"`
}
type Binding struct {
	Resource     string         `json:"resource"`
	Params       map[string]any `json:"params,omitempty"`
	InputsSchema Schema         `json:"inputsSchema,omitempty"`
	Query        *Query         `json:"query,omitempty"`
	Select       []string       `json:"select,omitempty"`
}
type Query struct {
	Where   *Predicate `json:"where,omitempty"`
	OrderBy []Order    `json:"orderBy,omitempty"`
	Limit   int        `json:"limit,omitempty"`
	Cursor  *string    `json:"cursor,omitempty"`
}
type Predicate struct {
	And   []Predicate `json:"and,omitempty"`
	Or    []Predicate `json:"or,omitempty"`
	Field string      `json:"field,omitempty"`
	Op    string      `json:"op,omitempty"`
	Value any         `json:"value"`
}

func (p Predicate) MarshalJSON() ([]byte, error) {
	value := map[string]any{}
	if p.And != nil {
		value["and"] = p.And
	}
	if p.Or != nil {
		value["or"] = p.Or
	}
	if p.Op != "" || p.Field != "" {
		value["field"] = p.Field
		value["op"] = p.Op
		value["value"] = p.Value
	}
	return json.Marshal(value)
}

type Order struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}
type Record struct {
	ID            string          `json:"id"`
	Revision      string          `json:"revision"`
	SchemaVersion int64           `json:"schemaVersion"`
	Data          json.RawMessage `json:"data"`
}
type Event struct {
	SourceUpdatedAt string   `json:"sourceUpdatedAt,omitempty"`
	Protocol        string   `json:"protocol"`
	SubscriptionID  string   `json:"subscriptionId"`
	Resource        string   `json:"resource"`
	Epoch           string   `json:"epoch"`
	Seq             string   `json:"seq"`
	Op              string   `json:"op"`
	Records         []Record `json:"records,omitempty"`
	Complete        bool     `json:"complete,omitempty"`
	NextCursor      string   `json:"nextCursor,omitempty"`
	Record          *Record  `json:"record,omitempty"`
	ID              string   `json:"id,omitempty"`
	Revision        string   `json:"revision,omitempty"`
	Reason          string   `json:"reason,omitempty"`
}
type Command struct {
	Op           string          `json:"op"`
	Resource     string          `json:"resource"`
	ID           string          `json:"id,omitempty"`
	Data         json.RawMessage `json:"data,omitempty"`
	BaseRevision string          `json:"baseRevision,omitempty"`
	RequestID    string          `json:"requestId"`
	ContractHash string          `json:"contractHash"`
}

func (e Event) MarshalJSON() ([]byte, error) {
	type envelope Event
	if e.Op != "snapshot" {
		return json.Marshal(envelope(e))
	}
	records := e.Records
	if records == nil {
		records = []Record{}
	}
	// Empty snapshots must retain records:[] and the explicit completeness flag.
	return json.Marshal(struct {
		envelope
		Records  []Record `json:"records"`
		Complete bool     `json:"complete"`
	}{envelope(e), records, e.Complete})
}

// Provider profiles are trusted registration metadata, not authored grants.
type ProviderProfile struct {
	Version      int
	Operations   []string
	Observation  string // "" | ordered-changes | refresh-snapshots
	Owned        bool
	ParamsSchema Schema
}
type Registry map[string]ProviderProfile

type Compiled struct {
	Contract      Contract
	Hash          string // SHA-256 of the exact validated source bytes.
	Source        json.RawMessage
	BindingOrder  []string
	OutputSchemas map[string]Schema
}
