package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestCheckHealthWithValidViewerToken(t *testing.T) {
	const token = "viewer-test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/services" || r.URL.Query().Get("limit") != "1" {
			t.Errorf("unexpected health request: %s %s", r.Method, r.URL.String())
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("unexpected authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"services":[],"hasMore":false}`))
	}))
	defer server.Close()

	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest(server.URL, token))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusOk {
		t.Fatalf("expected healthy result, got %q: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "authenticated viewer API access verified") {
		t.Fatalf("unexpected success message: %s", result.Message)
	}
}

func TestCheckHealthRejectsInvalidTokenWithoutLeakingIt(t *testing.T) {
	const token = "invalid-viewer-test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("unexpected authorization header")
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"rejected ` + token + `"}`))
	}))
	defer server.Close()

	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest(server.URL, token))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "HTTP 401") {
		t.Fatalf("expected an authentication error, got %q: %s", result.Status, result.Message)
	}
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), token) {
		t.Fatalf("health check result leaked the viewer token: %s", string(serialized))
	}
}

func TestCheckHealthReportsForbiddenTokenWithoutLeakingIt(t *testing.T) {
	const token = "forbidden-viewer-test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("unexpected authorization header")
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden ` + token + `"}`))
	}))
	defer server.Close()

	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest(server.URL, token))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "HTTP 403") {
		t.Fatalf("expected a forbidden error, got %q: %s", result.Status, result.Message)
	}
}

func TestCheckHealthRejectsMalformedURL(t *testing.T) {
	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest("http://", "token"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "absolute HTTP or HTTPS") {
		t.Fatalf("expected URL validation error, got %q: %s", result.Status, result.Message)
	}
}

func TestCheckHealthRejectsMissingToken(t *testing.T) {
	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest("http://coremetry:8088", ""))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "token is not configured") {
		t.Fatalf("expected missing token error, got %q: %s", result.Status, result.Message)
	}
}

func TestCheckHealthFailsOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, err := (&Datasource{}).CheckHealth(ctx, healthRequest(server.URL, "token"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "Unable to reach Coremetry") {
		t.Fatalf("expected reachability error on timeout, got %q: %s", result.Status, result.Message)
	}
}

func TestCheckHealthRefusesRedirects(t *testing.T) {
	var redirects atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirects.Add(1)
		http.Redirect(w, r, "/api/services?limit=1", http.StatusFound)
	}))
	defer server.Close()

	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest(server.URL, "token"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError {
		t.Fatalf("expected error on redirect, got %q: %s", result.Status, result.Message)
	}
	if redirects.Load() != 1 {
		t.Fatalf("expected only 1 request without following redirect, got %d", redirects.Load())
	}
}

func TestQueryData_RangeQuery(t *testing.T) {
	const token = "viewer-token-123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("unexpected authorization header: %s", r.Header.Get("Authorization"))
		}
		if !strings.Contains(r.URL.Path, "/api/metrics/promql") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query().Get("query")
		if q != "http.server.requests" {
			t.Errorf("unexpected query: %s", q)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"series": [
				{
					"groupKey": ["service=web-bff"],
					"points": [
						{"time": 1794610000000000000, "value": 15.5},
						{"time": 1794610015000000000, "value": 25.5}
					]
				}
			],
			"meta": {
				"source": "clickhouse",
				"executed": "http.server.requests"
			}
		}`))
	}))
	defer server.Close()

	ds := &Datasource{}
	req := queryDataRequest(server.URL, token, []backend.DataQuery{
		{
			RefID: "A",
			JSON:  []byte(`{"rawQuery":"http.server.requests","range":true}`),
		},
	})

	resp, err := ds.QueryData(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resA, ok := resp.Responses["A"]
	if !ok {
		t.Fatalf("missing response for query A")
	}
	if resA.Error != nil {
		t.Fatalf("unexpected query error: %v", resA.Error)
	}
	if len(resA.Frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(resA.Frames))
	}
	if resA.Frames[0].Rows() != 2 {
		t.Fatalf("expected 2 rows, got %d", resA.Frames[0].Rows())
	}
}

func TestQueryData_MultiQueryConcurrent(t *testing.T) {
	const token = "viewer-token-123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		if q == "queryA" {
			w.Write([]byte(`{"series":[{"groupKey":["a"],"points":[{"time":1000,"value":1}]}]}`))
		} else {
			w.Write([]byte(`{"series":[{"groupKey":["b"],"points":[{"time":1000,"value":2}]}]}`))
		}
	}))
	defer server.Close()

	ds := &Datasource{}
	req := queryDataRequest(server.URL, token, []backend.DataQuery{
		{RefID: "A", JSON: []byte(`{"rawQuery":"queryA"}`)},
		{RefID: "B", JSON: []byte(`{"rawQuery":"queryB"}`)},
	})

	resp, err := ds.QueryData(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resp.Responses))
	}
	if resp.Responses["A"].Frames[0].Fields[1].At(0).(float64) != 1 {
		t.Errorf("expected A value 1, got %v", resp.Responses["A"].Frames[0].Fields[1].At(0))
	}
	if resp.Responses["B"].Frames[0].Fields[1].At(0).(float64) != 2 {
		t.Errorf("expected B value 2, got %v", resp.Responses["B"].Frames[0].Fields[1].At(0))
	}
}

func TestQueryData_APMQueries(t *testing.T) {
	const token = "viewer-token-123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case path == "/api/services":
			w.Write([]byte(`{"services":[{"name":"api-gateway","health":"green","errorRate":0.1}]}`))
		case strings.HasSuffix(path, "/metric-red"):
			w.Write([]byte(`{"service":"api-gateway","series":{"rate":[{"groupKey":[],"points":[{"time":1000,"value":10.0}]}]}}`))
		case path == "/api/logs":
			w.Write([]byte(`{"logs":[{"id":1,"timestamp":1000,"severityText":"ERROR","body":"database connection timeout","serviceName":"auth-service","traceId":"trace-1"}]}`))
		case path == "/api/traces":
			w.Write([]byte(`{"traces":[{"traceId":"trace-1","serviceName":"auth-service","rootName":"POST /login","durationMs":25.0,"spanCount":3,"hasError":true}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ds := &Datasource{}

	// 1. Services
	reqServices := queryDataRequest(server.URL, token, []backend.DataQuery{
		{RefID: "Svc", JSON: []byte(`{"queryType":"services"}`)},
	})
	resp, err := ds.QueryData(context.Background(), reqServices)
	if err != nil || resp.Responses["Svc"].Error != nil {
		t.Fatalf("services query failed: %v", resp.Responses["Svc"].Error)
	}
	if resp.Responses["Svc"].Frames[0].Rows() != 1 {
		t.Errorf("expected 1 service row, got %d", resp.Responses["Svc"].Frames[0].Rows())
	}

	// 2. RED
	reqRED := queryDataRequest(server.URL, token, []backend.DataQuery{
		{RefID: "Red", JSON: []byte(`{"queryType":"red","service":"api-gateway","redMetric":"rate"}`)},
	})
	resp, err = ds.QueryData(context.Background(), reqRED)
	if err != nil || resp.Responses["Red"].Error != nil {
		t.Fatalf("red query failed: %v", resp.Responses["Red"].Error)
	}
	if len(resp.Responses["Red"].Frames) != 1 {
		t.Errorf("expected 1 red frame, got %d", len(resp.Responses["Red"].Frames))
	}

	// 3. Logs
	reqLogs := queryDataRequest(server.URL, token, []backend.DataQuery{
		{RefID: "Log", JSON: []byte(`{"queryType":"logs","service":"auth-service"}`)},
	})
	resp, err = ds.QueryData(context.Background(), reqLogs)
	if err != nil || resp.Responses["Log"].Error != nil {
		t.Fatalf("logs query failed: %v", resp.Responses["Log"].Error)
	}
	if resp.Responses["Log"].Frames[0].Rows() != 1 {
		t.Errorf("expected 1 log row, got %d", resp.Responses["Log"].Frames[0].Rows())
	}

	// 4. Traces
	reqTraces := queryDataRequest(server.URL, token, []backend.DataQuery{
		{RefID: "Traces", JSON: []byte(`{"queryType":"traces","service":"auth-service"}`)},
	})
	resp, err = ds.QueryData(context.Background(), reqTraces)
	if err != nil || resp.Responses["Traces"].Error != nil {
		t.Fatalf("traces query failed: %v", resp.Responses["Traces"].Error)
	}
	if resp.Responses["Traces"].Frames[0].Rows() != 1 {
		t.Errorf("expected 1 trace row, got %d", resp.Responses["Traces"].Frames[0].Rows())
	}
}

type mockCallResourceSender struct {
	resp *backend.CallResourceResponse
}

func (m *mockCallResourceSender) Send(resp *backend.CallResourceResponse) error {
	m.resp = resp
	return nil
}

func TestCallResource(t *testing.T) {
	const token = "viewer-token-123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/service-names" {
			w.Write([]byte(`{"names":["auth-service","api-gateway"],"total":2,"hasMore":false}`))
		} else if r.URL.Path == "/api/operation-names" {
			w.Write([]byte(`{"names":["POST /login","GET /profile"],"total":2,"hasMore":false}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ds := &Datasource{}

	// Test /services
	senderSvc := &mockCallResourceSender{}
	err := ds.CallResource(context.Background(), &backend.CallResourceRequest{
		PluginContext: backend.PluginContext{
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
				URL:                     server.URL,
				DecryptedSecureJSONData: map[string]string{"apiToken": token},
			},
		},
		Path: "services",
	}, senderSvc)
	if err != nil || senderSvc.resp.Status != http.StatusOK {
		t.Fatalf("CallResource /services failed: %v, status: %d", err, senderSvc.resp.Status)
	}
	if !strings.Contains(string(senderSvc.resp.Body), "auth-service") {
		t.Errorf("expected body to contain auth-service, got %s", string(senderSvc.resp.Body))
	}

	// Test /operations
	senderOp := &mockCallResourceSender{}
	err = ds.CallResource(context.Background(), &backend.CallResourceRequest{
		PluginContext: backend.PluginContext{
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
				URL:                     server.URL,
				DecryptedSecureJSONData: map[string]string{"apiToken": token},
			},
		},
		Path: "operations",
		URL:  "/operations?service=auth-service",
	}, senderOp)
	if err != nil || senderOp.resp.Status != http.StatusOK {
		t.Fatalf("CallResource /operations failed: %v, status: %d", err, senderOp.resp.Status)
	}
	if !strings.Contains(string(senderOp.resp.Body), "POST /login") {
		t.Errorf("expected body to contain POST /login, got %s", string(senderOp.resp.Body))
	}
}

func TestQueryData_HandlesErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"promql: syntax error"}`))
	}))
	defer server.Close()

	ds := &Datasource{}
	req := queryDataRequest(server.URL, "token", []backend.DataQuery{
		{RefID: "A", JSON: []byte(`{"rawQuery":"bad query"}`)},
	})

	resp, err := ds.QueryData(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Responses["A"].Error == nil || !strings.Contains(resp.Responses["A"].Error.Error(), "promql: syntax error") {
		t.Fatalf("expected syntax error message, got %v", resp.Responses["A"].Error)
	}
}

func TestQueryData_EmptyQueryReturnsEmptyFrame(t *testing.T) {
	ds := &Datasource{}
	req := queryDataRequest("http://localhost:8088", "token", []backend.DataQuery{
		{RefID: "A", JSON: []byte(`{"rawQuery":""}`)},
	})

	resp, err := ds.QueryData(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resA := resp.Responses["A"]
	if resA.Error != nil {
		t.Fatalf("unexpected error: %v", resA.Error)
	}
	if len(resA.Frames) != 1 || resA.Frames[0].Rows() != 0 {
		t.Fatalf("expected 1 empty frame, got %d frames", len(resA.Frames))
	}
}

func queryDataRequest(baseURL, token string, queries []backend.DataQuery) *backend.QueryDataRequest {
	from := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	for i := range queries {
		if queries[i].TimeRange.From.IsZero() {
			queries[i].TimeRange = backend.TimeRange{From: from, To: to}
		}
	}
	return &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
				URL: baseURL,
				DecryptedSecureJSONData: map[string]string{
					"apiToken": token,
				},
			},
		},
		Queries: queries,
	}
}

func healthRequest(baseURL, token string) *backend.CheckHealthRequest {
	return &backend.CheckHealthRequest{
		PluginContext: backend.PluginContext{
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
				URL: baseURL,
				DecryptedSecureJSONData: map[string]string{
					"apiToken": token,
				},
			},
		},
	}
}
