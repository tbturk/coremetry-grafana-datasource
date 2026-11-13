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
	if strings.Contains(result.Message, token) || strings.Contains(string(serialized), token) {
		t.Fatal("health result exposed the API token")
	}
}

func TestCheckHealthReportsInsufficientViewerRole(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer server.Close()

	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest(server.URL, "viewer-test-token"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "lacks viewer access (HTTP 403)") {
		t.Fatalf("expected insufficient-role error, got %q: %s", result.Status, result.Message)
	}
}

func TestCheckHealthRequiresToken(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest(server.URL, ""))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "token is not configured") {
		t.Fatalf("expected missing-token error, got %q: %s", result.Status, result.Message)
	}
	if requests.Load() != 0 {
		t.Fatal("health check made a request without a token")
	}
}

func TestCheckHealthRejectsMalformedServicesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"services":{}}`))
	}))
	defer server.Close()

	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest(server.URL, "viewer-test-token"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "services must be an array") {
		t.Fatalf("expected invalid response error, got %q: %s", result.Status, result.Message)
	}
}

func TestCheckHealthHandlesClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"services":[]}`))
	}))
	defer server.Close()

	ds := &Datasource{client: &http.Client{Timeout: time.Millisecond}}
	result, err := ds.CheckHealth(context.Background(), healthRequest(server.URL, "viewer-test-token"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "Unable to reach Coremetry") {
		t.Fatalf("expected timeout error, got %q: %s", result.Status, result.Message)
	}
}

func TestCheckHealthDoesNotFollowRedirectsWithCredentials(t *testing.T) {
	var targetRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetRequests.Add(1)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest(redirector.URL, "viewer-test-token"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "HTTP 302") {
		t.Fatalf("expected redirect rejection, got %q: %s", result.Status, result.Message)
	}
	if targetRequests.Load() != 0 {
		t.Fatal("health check followed a redirect")
	}
}

func TestCheckHealthRejectsCredentialsEmbeddedInURL(t *testing.T) {
	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest("http://user:password@example.test", "viewer-test-token"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "absolute HTTP or HTTPS") {
		t.Fatalf("expected URL validation error, got %q: %s", result.Status, result.Message)
	}
	if strings.Contains(result.Message, "password") {
		t.Fatal("health result exposed URL credentials")
	}
}

func TestCheckHealthRejectsMalformedURL(t *testing.T) {
	result, err := (&Datasource{}).CheckHealth(context.Background(), healthRequest("not a URL", "viewer-test-token"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, "absolute HTTP or HTTPS") {
		t.Fatalf("expected URL validation error, got %q: %s", result.Status, result.Message)
	}
}

func TestQueryDataIsExplicitlyUnavailableInPhase2(t *testing.T) {
	response, err := (&Datasource{}).QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{RefID: "A"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := response.Responses["A"]
	if !ok {
		t.Fatal("expected a response for query A")
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "not available in Phase 2") {
		t.Fatalf("expected explicit Phase 2 response, got %v", result.Error)
	}
	if len(result.Frames) != 0 {
		t.Fatal("Phase 2 query stub must not return sample frames")
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
