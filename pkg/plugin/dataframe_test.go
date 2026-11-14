package plugin

import (
	"testing"
	"time"

	"github.com/cosretr/coremetry-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func TestExtractByLabels(t *testing.T) {
	tests := []struct {
		query    string
		expected []string
	}{
		{"http.server.requests", nil},
		{"sum by (service_name) (rate(http.server.requests[5m]))", []string{"service_name"}},
		{"sum by(service_name, status) (rate(http.server.requests[5m]))", []string{"service_name", "status"}},
		{"AVG BY (env) (cpu_usage)", []string{"env"}},
	}

	for _, tt := range tests {
		got := extractByLabels(tt.query)
		if len(got) != len(tt.expected) {
			t.Fatalf("for %q, expected %v, got %v", tt.query, tt.expected, got)
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("for %q, expected %v, got %v", tt.query, tt.expected, got)
			}
		}
	}
}

func TestSeriesToRangeFrames(t *testing.T) {
	t.Run("empty series produces empty frame", func(t *testing.T) {
		frames := SeriesToRangeFrames("A", "http.requests", nil, nil)
		if len(frames) != 1 {
			t.Fatalf("expected 1 frame, got %d", len(frames))
		}
		if frames[0].RefID != "A" {
			t.Errorf("expected RefID 'A', got %q", frames[0].RefID)
		}
		if frames[0].Rows() != 0 {
			t.Errorf("expected 0 rows, got %d", frames[0].Rows())
		}
	})

	t.Run("converts nanosecond points and preserves labels", func(t *testing.T) {
		now := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
		ns1 := now.UnixNano()
		ns2 := now.Add(time.Minute).UnixNano()

		series := []models.SpanMetricSeries{
			{
				GroupKey: []string{"web-bff"},
				Points: []models.SpanMetricPoint{
					{Time: ns1, Value: 10.5},
					{Time: ns2, Value: 20.5},
				},
			},
			{
				GroupKey: []string{"service=api-gateway", "env=prod"},
				Points: []models.SpanMetricPoint{
					{Time: ns1, Value: 5.0},
					{Time: ns2, Value: 15.0},
				},
			},
		}

		query := "sum by (service_name) (rate(http.requests[5m]))"
		frames := SeriesToRangeFrames("A", query, series, &models.SpanMetricMeta{Executed: query})

		if len(frames) != 2 {
			t.Fatalf("expected 2 frames, got %d", len(frames))
		}

		// Check first frame
		f1 := frames[0]
		if f1.Rows() != 2 {
			t.Errorf("expected 2 rows, got %d", f1.Rows())
		}
		t1 := f1.Fields[0].At(0).(time.Time)
		if !t1.Equal(now) {
			t.Errorf("expected time %v, got %v", now, t1)
		}
		v1 := f1.Fields[1].At(0).(float64)
		if v1 != 10.5 {
			t.Errorf("expected value 10.5, got %v", v1)
		}
		if f1.Fields[1].Labels["service_name"] != "web-bff" {
			t.Errorf("expected service_name='web-bff', got %v", f1.Fields[1].Labels)
		}

		// Check second frame
		f2 := frames[1]
		if f2.Fields[1].Labels["service"] != "api-gateway" || f2.Fields[1].Labels["env"] != "prod" {
			t.Errorf("expected key=value labels preserved, got %v", f2.Fields[1].Labels)
		}
	})
}

func TestSeriesToInstantFrames(t *testing.T) {
	t.Run("converts instant points picking latest point", func(t *testing.T) {
		t1 := time.Date(2026, 11, 14, 2, 0, 0, 0, time.UTC)
		t2 := time.Date(2026, 11, 14, 2, 1, 0, 0, time.UTC)

		series := []models.SpanMetricSeries{
			{
				GroupKey: []string{"account-service"},
				Points: []models.SpanMetricPoint{
					{Time: t1.UnixNano(), Value: 100},
					{Time: t2.UnixNano(), Value: 250},
				},
			},
		}

		frames := SeriesToInstantFrames("B", "sum by (service) (http.requests)", t2, series, nil)
		if len(frames) != 1 {
			t.Fatalf("expected 1 frame, got %d", len(frames))
		}
		f := frames[0]
		if f.Rows() != 1 {
			t.Fatalf("expected 1 row for instant query, got %d", f.Rows())
		}
		val := f.Fields[1].At(0).(float64)
		if val != 250 {
			t.Errorf("expected latest value 250, got %v", val)
		}
		timeVal := f.Fields[0].At(0).(time.Time)
		if !timeVal.Equal(t2) {
			t.Errorf("expected time %v, got %v", t2, timeVal)
		}
	})
}

func TestServicesToDataFrame(t *testing.T) {
	services := []models.ServiceSummary{
		{
			Name:          "api-gateway",
			Health:        "green",
			SpanCount:     15000,
			ErrorCount:    5,
			ErrorRate:     0.033,
			AvgDurationMs: 12.5,
			P99DurationMs: 120.0,
			Apdex:         0.99,
			LastSeen:      1794614214000000000,
		},
	}

	frame := ServicesToDataFrame(services, "ServicesRef")
	if frame.RefID != "ServicesRef" {
		t.Errorf("expected RefID 'ServicesRef', got %q", frame.RefID)
	}
	if frame.Rows() != 1 {
		t.Fatalf("expected 1 row, got %d", frame.Rows())
	}

	nameVal := frame.Fields[0].At(0).(string)
	if nameVal != "api-gateway" {
		t.Errorf("expected service name 'api-gateway', got %q", nameVal)
	}
	errRateVal := frame.Fields[4].At(0).(float64)
	if errRateVal != 0.033 {
		t.Errorf("expected error rate 0.033, got %v", errRateVal)
	}
}

func TestREDResponseToDataFrames(t *testing.T) {
	now := time.Now()
	red := &models.ServiceREDResponse{
		Service: "coremetry-monolithic",
		Series: map[string][]models.SpanMetricSeries{
			"rate": {
				{
					GroupKey: []string{},
					Points: []models.SpanMetricPoint{
						{Time: now.UnixNano(), Value: 42.5},
					},
				},
			},
			"p99": {
				{
					GroupKey: []string{},
					Points: []models.SpanMetricPoint{
						{Time: now.UnixNano(), Value: 185.0},
					},
				},
			},
		},
	}

	t.Run("all metrics", func(t *testing.T) {
		frames := REDResponseToDataFrames(red, "all", "R")
		if len(frames) != 2 {
			t.Fatalf("expected 2 frames (rate and p99), got %d", len(frames))
		}
	})

	t.Run("filtered to rate", func(t *testing.T) {
		frames := REDResponseToDataFrames(red, "rate", "R")
		if len(frames) != 1 {
			t.Fatalf("expected 1 frame for rate, got %d", len(frames))
		}
		if frames[0].Fields[1].Config.Unit != "reqps" {
			t.Errorf("expected unit 'reqps', got %q", frames[0].Fields[1].Config.Unit)
		}
	})
}

func TestLogsToDataFrame(t *testing.T) {
	logs := []*models.LogRecord{
		{
			ID:           1,
			Timestamp:    1794614856000000000,
			Severity:     9,
			SeverityText: "INFO",
			Body:         "user login succeeded",
			ServiceName:  "auth-service",
			TraceID:      "trace-12345",
			SpanID:       "span-67890",
		},
	}

	frame := LogsToDataFrame(logs, "LogsRef", "coremetry-tempo")
	if frame.RefID != "LogsRef" {
		t.Errorf("expected RefID 'LogsRef', got %q", frame.RefID)
	}
	if frame.Meta == nil || frame.Meta.PreferredVisualization != data.VisTypeLogs {
		t.Errorf("expected PreferredVisualization to be VisTypeLogs")
	}
	if frame.Rows() != 1 {
		t.Fatalf("expected 1 row, got %d", frame.Rows())
	}

	// Verify traceId field has data link pointing to Tempo datasource
	traceField := frame.Fields[4]
	if traceField.Name != "traceId" {
		t.Fatalf("expected field 4 to be traceId, got %q", traceField.Name)
	}
	if len(traceField.Config.Links) == 0 {
		t.Fatalf("expected traceId field to have data link")
	}
	link := traceField.Config.Links[0]
	if link.Title != "View trace in Tempo" {
		t.Errorf("expected link title 'View trace in Tempo', got %q", link.Title)
	}
}

func TestTracesToDataFrame(t *testing.T) {
	traces := []models.TraceSummary{
		{
			TraceID:     "trace-abc",
			RootName:    "GET /users",
			ServiceName: "user-service",
			StartTime:   1794614856000000000,
			DurationMs:  45.2,
			SpanCount:   4,
			HasError:    false,
		},
	}

	frame := TracesToDataFrame(traces, "TracesRef", "coremetry-tempo")
	if frame.RefID != "TracesRef" {
		t.Errorf("expected RefID 'TracesRef', got %q", frame.RefID)
	}
	if frame.Rows() != 1 {
		t.Fatalf("expected 1 row, got %d", frame.Rows())
	}
	traceField := frame.Fields[1]
	if traceField.Name != "traceId" {
		t.Fatalf("expected field 1 to be traceId, got %q", traceField.Name)
	}
	if len(traceField.Config.Links) == 0 {
		t.Fatalf("expected traceId field to have data link")
	}
}
