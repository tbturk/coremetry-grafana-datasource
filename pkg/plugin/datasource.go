package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cosretr/coremetry-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
)

const (
	healthCheckTimeout = 5 * time.Second
	healthResponseMax  = 1 << 20
)

var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

type Datasource struct {
	client *http.Client
}

func NewDatasource(_ context.Context, _ backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	return &Datasource{
		client: newHTTPClient(),
	}, nil
}

func (d *Datasource) Dispose() {}

func (d *Datasource) QueryData(_ context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()
	for _, query := range req.Queries {
		response.Responses[query.RefID] = backend.ErrDataResponse(
			backend.StatusBadRequest,
			"Metric query execution is not available in Phase 2.",
		)
	}
	return response, nil
}

func (d *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if req == nil || req.PluginContext.DataSourceInstanceSettings == nil {
		return healthError("Datasource settings are unavailable.")
	}

	settings, err := models.LoadPluginSettings(*req.PluginContext.DataSourceInstanceSettings)
	if err != nil {
		return healthError("Datasource settings are invalid.")
	}
	if strings.TrimSpace(settings.APIToken) == "" {
		return healthError("Coremetry viewer API token is not configured.")
	}

	healthURL, err := coremetryServicesURL(settings.BaseURL)
	if err != nil {
		return healthError("Coremetry URL must be an absolute HTTP or HTTPS base URL.")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return healthError("Unable to create the Coremetry health request.")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+settings.APIToken)

	client := d.client
	if client == nil {
		client = newHTTPClient()
	}
	response, err := client.Do(request)
	if err != nil {
		return healthError("Unable to reach Coremetry. Check the URL and Docker network.")
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized {
		return healthError("Coremetry rejected the API token (HTTP 401).")
	}
	if response.StatusCode == http.StatusForbidden {
		return healthError("The token is valid but lacks viewer access (HTTP 403).")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return healthError(fmt.Sprintf("Coremetry returned HTTP %d for the authenticated health check.", response.StatusCode))
	}

	var payload struct {
		Services json.RawMessage `json:"services"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, healthResponseMax)).Decode(&payload); err != nil {
		return healthError("Coremetry returned an invalid health-check response.")
	}
	if len(payload.Services) == 0 {
		return healthError("Coremetry returned an unexpected response; expected a services field.")
	}
	var services []json.RawMessage
	if err := json.Unmarshal(payload.Services, &services); err != nil || services == nil {
		return healthError("Coremetry returned an unexpected response; services must be an array.")
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Connected to Coremetry; authenticated viewer API access verified.",
	}, nil
}

func coremetryServicesURL(base string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", errors.New("invalid Coremetry base URL")
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid Coremetry base URL")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/services"
	parsed.RawPath = ""
	query := parsed.Query()
	query.Set("limit", "1")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func healthError(message string) (*backend.CheckHealthResult, error) {
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusError,
		Message: message,
	}, nil
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: healthCheckTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
