package models

import "time"

// ServiceSummary represents an item from GET /api/services.
type ServiceSummary struct {
	Name             string  `json:"name"`
	SpanCount        int64   `json:"spanCount"`
	ErrorCount       int64   `json:"errorCount"`
	ErrorRate        float64 `json:"errorRate"`
	AvgDurationMs    float64 `json:"avgDurationMs"`
	P99DurationMs    float64 `json:"p99DurationMs"`
	Apdex            float64 `json:"apdex"`
	ApdexThresholdMs int64   `json:"apdexThresholdMs"`
	Health           string  `json:"health"`
	LastSeen         int64   `json:"lastSeen"`
	FirstSeen        int64   `json:"firstSeen"`
}

// ServicesResponse is the envelope for GET /api/services.
type ServicesResponse struct {
	Services []ServiceSummary `json:"services"`
	HasMore  bool             `json:"hasMore"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
}

// ServiceNamesResponse is the envelope for GET /api/service-names.
type ServiceNamesResponse struct {
	Names   []string `json:"names"`
	Total   int      `json:"total"`
	HasMore bool     `json:"hasMore"`
}

// OperationNamesResponse is the envelope for GET /api/operation-names.
type OperationNamesResponse struct {
	Names   []string `json:"names"`
	Total   int      `json:"total"`
	HasMore bool     `json:"hasMore"`
}

// ServiceREDResponse is the envelope for GET /api/services/{name}/metric-red.
type ServiceREDResponse struct {
	Service          string                    `json:"service"`
	Source           string                    `json:"source"`
	Metric           string                    `json:"metric,omitempty"`
	RTMetric         string                    `json:"rtMetric,omitempty"`
	MetricExists     bool                      `json:"metricExists"`
	StepSeconds      int                       `json:"stepSeconds"`
	ErrorsUnknown    bool                      `json:"errorsUnknown,omitempty"`
	LatencyUnit      string                    `json:"latencyUnit,omitempty"`
	LatencyUnitKnown bool                      `json:"latencyUnitKnown"`
	Note             string                    `json:"note"`
	Series           map[string][]SpanMetricSeries `json:"series"`
}

// LogRecord represents a single log line returned by GET /api/logs.
type LogRecord struct {
	ID                 int64             `json:"id"`
	Timestamp          int64             `json:"timestamp"` // Unix nanoseconds
	Severity           uint8             `json:"severity"`
	SeverityText       string            `json:"severityText"`
	Body               string            `json:"body"`
	ServiceName        string            `json:"serviceName"`
	TraceID            string            `json:"traceId"`
	SpanID             string            `json:"spanId"`
	Attributes         map[string]string `json:"attributes"`
	ResourceAttributes map[string]string `json:"resourceAttributes"`
}

// LogsResponse is the envelope for GET /api/logs.
type LogsResponse struct {
	Total      int          `json:"total"`
	Logs       []*LogRecord `json:"logs"`
	NextCursor string       `json:"nextCursor,omitempty"`
	Partial    bool         `json:"partial,omitempty"`
	Degraded   bool         `json:"degraded,omitempty"`
}

// LogFilter contains the parameters for querying logs.
type LogFilter struct {
	Service  string
	Search   string
	Severity string
	TraceID  string
	SpanID   string
	From     time.Time
	To       time.Time
	Limit    int
}

// TraceSummary represents a trace search hit from GET /api/traces.
type TraceSummary struct {
	TraceID     string  `json:"traceId"`
	RootName    string  `json:"rootName"`
	ServiceName string  `json:"serviceName"`
	StartTime   int64   `json:"startTime"` // Unix nanoseconds
	DurationMs  float64 `json:"durationMs"`
	SpanCount   int     `json:"spanCount"`
	HasError    bool    `json:"hasError"`
}

// TracesResponse is the envelope for GET /api/traces.
type TracesResponse struct {
	Traces  []TraceSummary `json:"traces"`
	HasMore bool           `json:"hasMore"`
}

// TraceFilter contains the parameters for querying traces.
type TraceFilter struct {
	Service string
	TraceID string
	Search  string
	From    time.Time
	To      time.Time
	Limit   int
}
