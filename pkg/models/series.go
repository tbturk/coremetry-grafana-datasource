package models

type SpanMetricPoint struct {
	Time  int64   `json:"time"` // unix nanoseconds
	Value float64 `json:"value"`
}

type SpanMetricSeries struct {
	GroupKey []string          `json:"groupKey"`
	Points   []SpanMetricPoint `json:"points"`
}

type SpanMetricMeta struct {
	Source      string `json:"source,omitempty"`
	Executed    string `json:"executed,omitempty"`
	TookMs      int64  `json:"tookMs,omitempty"`
	SeriesTotal int    `json:"seriesTotal,omitempty"`
	SeriesShown int    `json:"seriesShown,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
	Partial     bool   `json:"partial,omitempty"`
	StepSec     int    `json:"stepSec,omitempty"`
}

type PromQLResponse struct {
	Series []SpanMetricSeries `json:"series"`
	Meta   *SpanMetricMeta    `json:"meta,omitempty"`
}

type APIErrorResponse struct {
	Error  string `json:"error"`
	Pos    int    `json:"pos,omitempty"`
	Line   int    `json:"line,omitempty"`
	Col    int    `json:"col,omitempty"`
	Detail string `json:"detail,omitempty"`
}
