package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cosretr/coremetry-grafana-datasource/pkg/models"
)

func TestBuildPromQLURL(t *testing.T) {
	t.Run("valid parameters", func(t *testing.T) {
		u, err := buildPromQLURL("http://localhost:8088", "http.requests", 1000, 2000, 15, 300)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "http://localhost:8088/api/metrics/promql?from=1000&maxDataPoints=300&meta=1&query=http.requests&step=15&to=2000"
		if u != expected {
			t.Errorf("expected %q, got %q", expected, u)
		}
	})

	t.Run("clamps maxDataPoints to 4000", func(t *testing.T) {
		u, err := buildPromQLURL("http://localhost:8088", "up", 0, 0, 0, 5000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u != "http://localhost:8088/api/metrics/promql?maxDataPoints=4000&meta=1&query=up" {
			t.Errorf("expected clamped maxDataPoints 4000, got %q", u)
		}
	})

	t.Run("invalid url", func(t *testing.T) {
		_, err := buildPromQLURL("not-a-valid-url", "up", 0, 0, 0, 0)
		if err == nil {
			t.Errorf("expected error on invalid url, got nil")
		}
	})
}

func TestCalculateStep(t *testing.T) {
	from := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour) // 3600 seconds

	t.Run("explicit user step", func(t *testing.T) {
		step := calculateStep(from, to, "15s", 0, 0)
		if step != 15 {
			t.Errorf("expected step 15, got %d", step)
		}
	})

	t.Run("query interval", func(t *testing.T) {
		step := calculateStep(from, to, "", 30*time.Second, 0)
		if step != 30 {
			t.Errorf("expected step 30, got %d", step)
		}
	})

	t.Run("auto step from maxDataPoints", func(t *testing.T) {
		// 3600s / 100 = 36s
		step := calculateStep(from, to, "", 0, 100)
		if step != 36 {
			t.Errorf("expected step 36, got %d", step)
		}
	})
}

func TestCoremetryClient_QueryPromQL(t *testing.T) {
	t.Run("successful envelope response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer cmk_test" {
				t.Errorf("expected auth Bearer cmk_test, got %q", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"series":[{"groupKey":["web"],"points":[{"time":100,"value":42}]}],"meta":{"source":"clickhouse"}}`))
		}))
		defer server.Close()

		client := NewCoremetryClient(server.Client())
		resp, err := client.QueryPromQL(context.Background(), server.URL, "cmk_test", "up", 0, 0, 15, 300)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Series) != 1 {
			t.Fatalf("expected 1 series, got %d", len(resp.Series))
		}
		if resp.Series[0].Points[0].Value != 42 {
			t.Errorf("expected value 42, got %v", resp.Series[0].Points[0].Value)
		}
	})

	t.Run("handles 400 PromQL syntax error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"promql: expected ')', got \"\" (at position 14)"}`))
		}))
		defer server.Close()

		client := NewCoremetryClient(server.Client())
		_, err := client.QueryPromQL(context.Background(), server.URL, "cmk_test", "bad_query(", 0, 0, 15, 300)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err.Error() != `promql: expected ')', got ""`+" (at position 14)" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("handles 401 unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid token"}`))
		}))
		defer server.Close()

		client := NewCoremetryClient(server.Client())
		_, err := client.QueryPromQL(context.Background(), server.URL, "cmk_bad", "up", 0, 0, 15, 300)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err.Error() != "invalid token" {
			t.Errorf("expected 'invalid token', got %v", err)
		}
	})
}

func TestCoremetryClient_APMMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/services":
			w.Write([]byte(`{"services":[{"name":"api-gateway","health":"green"}],"limit":50,"offset":0}`))
		case "/api/service-names":
			w.Write([]byte(`{"names":["api-gateway","auth-service"],"total":2}`))
		case "/api/operation-names":
			w.Write([]byte(`{"names":["POST /login"],"total":1}`))
		case "/api/services/auth-service/metric-red":
			w.Write([]byte(`{"service":"auth-service","series":{"rate":[{"groupKey":[],"points":[{"time":100,"value":5}]}]}}`))
		case "/api/logs":
			w.Write([]byte(`{"total":1,"logs":[{"id":10,"timestamp":100,"body":"login success","serviceName":"auth-service"}]}`))
		case "/api/traces":
			w.Write([]byte(`{"traces":[{"traceId":"t1","serviceName":"auth-service","durationMs":15.0}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewCoremetryClient(server.Client())
	ctx := context.Background()

	// GetServices
	svcs, err := client.GetServices(ctx, server.URL, "token", 10)
	if err != nil || len(svcs.Services) != 1 {
		t.Fatalf("GetServices failed: %v", err)
	}

	// GetServiceNames
	names, err := client.GetServiceNames(ctx, server.URL, "token", "", 10)
	if err != nil || len(names.Names) != 2 {
		t.Fatalf("GetServiceNames failed: %v", err)
	}

	// GetOperationNames
	ops, err := client.GetOperationNames(ctx, server.URL, "token", "auth-service", 10)
	if err != nil || len(ops.Names) != 1 {
		t.Fatalf("GetOperationNames failed: %v", err)
	}

	// GetServiceMetricRED
	red, err := client.GetServiceMetricRED(ctx, server.URL, "token", "auth-service", time.Now().Add(-time.Hour), time.Now(), 100, 180)
	if err != nil || red.Service != "auth-service" {
		t.Fatalf("GetServiceMetricRED failed: %v", err)
	}

	// GetLogs
	logs, err := client.GetLogs(ctx, server.URL, "token", models.LogFilter{Service: "auth-service"})
	if err != nil || len(logs.Logs) != 1 {
		t.Fatalf("GetLogs failed: %v", err)
	}

	// GetTraces
	traces, err := client.GetTraces(ctx, server.URL, "token", models.TraceFilter{Service: "auth-service"})
	if err != nil || len(traces.Traces) != 1 {
		t.Fatalf("GetTraces failed: %v", err)
	}
}
