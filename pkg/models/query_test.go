package models

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestParseQuery(t *testing.T) {
	t.Run("default empty query", func(t *testing.T) {
		qm, err := ParseQuery(backend.DataQuery{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qm.QueryType != QueryTypeMetrics {
			t.Errorf("expected QueryType %q, got %q", QueryTypeMetrics, qm.QueryType)
		}
		if qm.RawQuery != "" {
			t.Errorf("expected empty rawQuery, got %q", qm.RawQuery)
		}
		if !qm.Range {
			t.Errorf("expected Range to be true by default")
		}
		if qm.Limit != 100 {
			t.Errorf("expected default limit 100, got %d", qm.Limit)
		}
	})

	t.Run("valid metrics query json", func(t *testing.T) {
		jsonBytes := []byte(`{"rawQuery":" http.server.requests ","instant":true,"range":false,"step":"30s"}`)
		qm, err := ParseQuery(backend.DataQuery{JSON: jsonBytes})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qm.QueryType != QueryTypeMetrics {
			t.Errorf("expected default QueryType 'metrics', got %q", qm.QueryType)
		}
		if qm.RawQuery != "http.server.requests" {
			t.Errorf("expected trimmed rawQuery 'http.server.requests', got %q", qm.RawQuery)
		}
		if !qm.Instant {
			t.Errorf("expected Instant to be true")
		}
		if qm.Range {
			t.Errorf("expected Range to be false")
		}
		if qm.Step != "30s" {
			t.Errorf("expected Step '30s', got %q", qm.Step)
		}
	})

	t.Run("valid APM queries", func(t *testing.T) {
		// Services query
		qServices, err := ParseQuery(backend.DataQuery{JSON: []byte(`{"queryType":"services"}`)})
		if err != nil || qServices.QueryType != QueryTypeServices {
			t.Fatalf("failed parsing services query: %v", err)
		}

		// RED query
		qRED, err := ParseQuery(backend.DataQuery{JSON: []byte(`{"queryType":"red","service":"api-gateway","redMetric":"error_rate"}`)})
		if err != nil || qRED.QueryType != QueryTypeRED || qRED.Service != "api-gateway" || qRED.REDMetric != "error_rate" {
			t.Fatalf("failed parsing RED query: %v", err)
		}

		// Logs query
		qLogs, err := ParseQuery(backend.DataQuery{JSON: []byte(`{"queryType":"logs","service":"auth-service","search":"login failed","severity":"17","limit":250}`)})
		if err != nil || qLogs.QueryType != QueryTypeLogs || qLogs.Service != "auth-service" || qLogs.Search != "login failed" || qLogs.Limit != 250 {
			t.Fatalf("failed parsing Logs query: %v", err)
		}

		// Traces query
		qTraces, err := ParseQuery(backend.DataQuery{JSON: []byte(`{"queryType":"traces","traceId":"1234567890abcdef"}`)})
		if err != nil || qTraces.QueryType != QueryTypeTraces || qTraces.TraceID != "1234567890abcdef" {
			t.Fatalf("failed parsing Traces query: %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		_, err := ParseQuery(backend.DataQuery{JSON: []byte(`{invalid}`)})
		if err == nil {
			t.Errorf("expected error on invalid json, got nil")
		}
	})
}
