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
	"sync"
	"time"

	"github.com/cosretr/coremetry-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const (
	healthCheckTimeout = 5 * time.Second
	healthResponseMax  = 1 << 20
)

var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ backend.CallResourceHandler   = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

type Datasource struct {
	client *CoremetryClient
}

func NewDatasource(_ context.Context, _ backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	return &Datasource{
		client: NewCoremetryClient(newHTTPClient()),
	}, nil
}

func (d *Datasource) Dispose() {}

func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()

	if req == nil || req.PluginContext.DataSourceInstanceSettings == nil {
		for _, q := range req.Queries {
			response.Responses[q.RefID] = backend.ErrDataResponse(
				backend.StatusBadRequest,
				"Datasource settings are unavailable.",
			)
		}
		return response, nil
	}

	settings, err := models.LoadPluginSettings(*req.PluginContext.DataSourceInstanceSettings)
	if err != nil {
		for _, q := range req.Queries {
			response.Responses[q.RefID] = backend.ErrDataResponse(
				backend.StatusBadRequest,
				"Datasource settings are invalid.",
			)
		}
		return response, nil
	}

	var pluginSettings models.CoremetryPluginSettings
	if len(req.PluginContext.DataSourceInstanceSettings.JSONData) > 0 {
		_ = json.Unmarshal(req.PluginContext.DataSourceInstanceSettings.JSONData, &pluginSettings)
	}
	traceDatasourceUID := pluginSettings.TraceDatasourceUID
	if traceDatasourceUID == "" {
		traceDatasourceUID = "coremetry-tempo"
	}

	client := d.client
	if client == nil {
		client = NewCoremetryClient(newHTTPClient())
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, query := range req.Queries {
		wg.Add(1)
		go func(q backend.DataQuery) {
			defer wg.Done()

			dataResp := d.querySingle(ctx, client, settings, q, traceDatasourceUID)

			mu.Lock()
			response.Responses[q.RefID] = dataResp
			mu.Unlock()
		}(query)
	}

	wg.Wait()
	return response, nil
}

func (d *Datasource) querySingle(
	ctx context.Context,
	client *CoremetryClient,
	settings *models.PluginSettings,
	query backend.DataQuery,
	traceDatasourceUID string,
) backend.DataResponse {
	qm, err := models.ParseQuery(query)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("failed to parse query: %v", err))
	}

	switch qm.QueryType {
	case models.QueryTypeServices:
		resp, err := client.GetServices(ctx, settings.BaseURL, settings.APIToken, qm.Limit)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
		}
		frame := ServicesToDataFrame(resp.Services, query.RefID)
		return backend.DataResponse{Frames: []*data.Frame{frame}}

	case models.QueryTypeRED:
		if qm.Service == "" {
			return backend.ErrDataResponse(backend.StatusBadRequest, "service name is required for RED metrics query")
		}
		resp, err := client.GetServiceMetricRED(ctx, settings.BaseURL, settings.APIToken, qm.Service, query.TimeRange.From, query.TimeRange.To, int(query.MaxDataPoints), 180)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
		}
		frames := REDResponseToDataFrames(resp, qm.REDMetric, query.RefID)
		return backend.DataResponse{Frames: frames}

	case models.QueryTypeLogs:
		filter := models.LogFilter{
			Service:  qm.Service,
			Search:   qm.Search,
			Severity: qm.Severity,
			TraceID:  qm.TraceID,
			From:     query.TimeRange.From,
			To:       query.TimeRange.To,
			Limit:    qm.Limit,
		}
		resp, err := client.GetLogs(ctx, settings.BaseURL, settings.APIToken, filter)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
		}
		frame := LogsToDataFrame(resp.Logs, query.RefID, traceDatasourceUID)
		return backend.DataResponse{Frames: []*data.Frame{frame}}

	case models.QueryTypeTraces:
		filter := models.TraceFilter{
			Service: qm.Service,
			TraceID: qm.TraceID,
			Search:  qm.Search,
			From:    query.TimeRange.From,
			To:      query.TimeRange.To,
			Limit:   qm.Limit,
		}
		resp, err := client.GetTraces(ctx, settings.BaseURL, settings.APIToken, filter)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
		}
		frame := TracesToDataFrame(resp.Traces, query.RefID, traceDatasourceUID)
		return backend.DataResponse{Frames: []*data.Frame{frame}}

	default: // metrics (PromQL)
		if qm.RawQuery == "" {
			return backend.DataResponse{
				Frames: SeriesToRangeFrames(query.RefID, "", nil, nil),
			}
		}

		timeRange := query.TimeRange
		stepSec := calculateStep(timeRange.From, timeRange.To, qm.Step, query.Interval, query.MaxDataPoints)

		var fromNs, toNs int64
		if qm.Instant {
			toNs = timeRange.To.UnixNano()
			lookbackSec := 15 * 60
			if stepSec*5 > lookbackSec {
				lookbackSec = stepSec * 5
			}
			fromNs = toNs - int64(lookbackSec)*1e9
			if fromNs < 0 {
				fromNs = 0
			}
		} else {
			fromNs = timeRange.From.UnixNano()
			toNs = timeRange.To.UnixNano()
		}

		promqlRes, err := client.QueryPromQL(ctx, settings.BaseURL, settings.APIToken, qm.RawQuery, fromNs, toNs, stepSec, int(query.MaxDataPoints))
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
		}

		if qm.Instant {
			frames := SeriesToInstantFrames(query.RefID, qm.RawQuery, timeRange.To, promqlRes.Series, promqlRes.Meta)
			return backend.DataResponse{Frames: frames}
		}

		frames := SeriesToRangeFrames(query.RefID, qm.RawQuery, promqlRes.Series, promqlRes.Meta)
		return backend.DataResponse{Frames: frames}
	}
}

func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	if req == nil || req.PluginContext.DataSourceInstanceSettings == nil {
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusBadRequest,
			Body:   []byte(`{"error":"Datasource settings are unavailable."}`),
		})
	}

	settings, err := models.LoadPluginSettings(*req.PluginContext.DataSourceInstanceSettings)
	if err != nil {
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusBadRequest,
			Body:   []byte(`{"error":"Datasource settings are invalid."}`),
		})
	}

	client := d.client
	if client == nil {
		client = NewCoremetryClient(newHTTPClient())
	}

	path := strings.TrimPrefix(strings.TrimSpace(req.Path), "/")
	parsedURL, _ := url.Parse(req.URL)
	queryVals := url.Values{}
	if parsedURL != nil {
		queryVals = parsedURL.Query()
	}

	switch {
	case path == "services":
		resp, err := client.GetServiceNames(ctx, settings.BaseURL, settings.APIToken, queryVals.Get("q"), 1000)
		if err != nil {
			return sender.Send(&backend.CallResourceResponse{
				Status: http.StatusBadRequest,
				Body:   []byte(fmt.Sprintf(`{"error":%q}`, err.Error())),
			})
		}
		dataBytes, _ := json.Marshal(resp)
		return sender.Send(&backend.CallResourceResponse{
			Status:  http.StatusOK,
			Headers: map[string][]string{"Content-Type": {"application/json"}},
			Body:    dataBytes,
		})

	case path == "operations":
		service := queryVals.Get("service")
		resp, err := client.GetOperationNames(ctx, settings.BaseURL, settings.APIToken, service, 1000)
		if err != nil {
			return sender.Send(&backend.CallResourceResponse{
				Status: http.StatusBadRequest,
				Body:   []byte(fmt.Sprintf(`{"error":%q}`, err.Error())),
			})
		}
		dataBytes, _ := json.Marshal(resp)
		return sender.Send(&backend.CallResourceResponse{
			Status:  http.StatusOK,
			Headers: map[string][]string{"Content-Type": {"application/json"}},
			Body:    dataBytes,
		})

	default:
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusNotFound,
			Body:   []byte(`{"error":"Resource not found"}`),
		})
	}
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
	var hc *http.Client
	if client != nil && client.httpClient != nil {
		hc = client.httpClient
	} else {
		hc = newHTTPClient()
	}

	response, err := hc.Do(request)
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
