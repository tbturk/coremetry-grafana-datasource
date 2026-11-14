package models

import (
	"encoding/json"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Supported query types.
const (
	QueryTypeMetrics  = "metrics"
	QueryTypeServices = "services"
	QueryTypeRED      = "red"
	QueryTypeLogs     = "logs"
	QueryTypeTraces   = "traces"
)

type QueryModel struct {
	QueryType string `json:"queryType"` // metrics, services, red, logs, traces
	RawQuery  string `json:"rawQuery"`  // PromQL expression
	Instant   bool   `json:"instant"`
	Range     bool   `json:"range"`
	Step      string `json:"step"`

	// APM Query fields
	Service   string `json:"service"`
	REDMetric string `json:"redMetric"` // all, rate, error_rate, p50, p95, p99, avg
	Search    string `json:"search"`
	Severity  string `json:"severity"`
	TraceID   string `json:"traceId"`
	Limit     int    `json:"limit"`
}

type CoremetryPluginSettings struct {
	TraceDatasourceUID string `json:"traceDatasourceUid"`
}

func ParseQuery(data backend.DataQuery) (*QueryModel, error) {
	qm := &QueryModel{
		QueryType: QueryTypeMetrics,
		Range:     true,
	}
	if len(data.JSON) > 0 {
		if err := json.Unmarshal(data.JSON, qm); err != nil {
			return nil, err
		}
	}
	if qm.QueryType == "" {
		qm.QueryType = QueryTypeMetrics
	}
	qm.RawQuery = strings.TrimSpace(qm.RawQuery)
	qm.Service = strings.TrimSpace(qm.Service)
	qm.TraceID = strings.TrimSpace(qm.TraceID)
	qm.Search = strings.TrimSpace(qm.Search)
	if qm.Limit <= 0 {
		qm.Limit = 100
	}
	return qm, nil
}
