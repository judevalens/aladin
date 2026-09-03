// Package compat contains the deliberately isolated Shard v1 compatibility
// boundary. New resource code must depend on shardresource.Service instead.
package compat

import (
	"log/slog"
	"sync"

	"aladin/backend_v2/internal/shardresource"
)

// V1RetirementCondition is intentionally executable documentation beside the
// telemetry: BR-048 may remove v1 only after production reports no calls for a
// full rollback window and every shipped client advertises bridge/2.
const V1RetirementCondition = "zero production v1 calls for one rollback window and all supported clients advertise bridge/2"

type V1Observer interface {
	Used(operation string, environment shardresource.Environment)
}

type LogV1Observer struct {
	logger *slog.Logger
	once   sync.Map
}

func NewLogV1Observer(logger *slog.Logger) *LogV1Observer {
	return &LogV1Observer{logger: logger}
}

func (o *LogV1Observer) Used(operation string, environment shardresource.Environment) {
	if o == nil || o.logger == nil {
		return
	}
	key := operation + ":" + string(environment)
	if _, loaded := o.once.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	o.logger.Info("shard v1 compatibility used",
		"operation", operation,
		"environment", environment,
		"retirement_condition", V1RetirementCondition,
	)
}
