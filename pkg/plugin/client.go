package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cosretr/coremetry-grafana-datasource/pkg/models"
)

const (
	defaultQueryTimeout = 30 * time.Second
	maxResponseBytes    = 10 << 20 // 10 MiB
)

type CoremetryClient struct {
	httpClient *http.Client
}

func NewCoremetryClient(httpClient *http.Client) *CoremetryClient {
	if httpClient == nil {
		httpClient = newHTTPClient()
	}
	return &CoremetryClient{httpClient: httpClient}
}

func (c *CoremetryClient) doJSONGet(ctx context.Context, rawURL, apiToken string, target any) error {
	if strings.TrimSpace(apiToken) == "" {
		return errors.New("Coremetry viewer API token is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		return fmt.Errorf("unable to reach Coremetry: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("failed to read Coremetry response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return parseAPIError(resp.StatusCode, bodyBytes)
	}

	if target != nil {
		if err := json.Unmarshal(bodyBytes, target); err != nil {
			return fmt.Errorf("failed to parse Coremetry response JSON: %w", err)
		}
	}

	return nil
}

func (c *CoremetryClient) QueryPromQL(
	ctx context.Context,
	baseURL string,
	apiToken string,
	promqlQuery string,
	fromNs int64,
	toNs int64,
	stepSec int,
	maxDP int,
) (*models.PromQLResponse, error) {
	if strings.TrimSpace(apiToken) == "" {
		return nil, errors.New("Coremetry viewer API token is not configured")
	}

	reqURL, err := buildPromQLURL(baseURL, promqlQuery, fromNs, toNs, stepSec, maxDP)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("unable to reach Coremetry at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read Coremetry response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp.StatusCode, bodyBytes)
	}

	var envelope models.PromQLResponse
	if err := json.Unmarshal(bodyBytes, &envelope); err == nil && (envelope.Series != nil || envelope.Meta != nil) {
		return &envelope, nil
	}

	var rawSeries []models.SpanMetricSeries
	if err := json.Unmarshal(bodyBytes, &rawSeries); err == nil {
		return &models.PromQLResponse{
			Series: rawSeries,
		}, nil
	}

	return nil, errors.New("Coremetry returned an unexpected response format")
}

func (c *CoremetryClient) GetServices(ctx context.Context, baseURL, apiToken string, limit int) (*models.ServicesResponse, error) {
	baseParsed, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	baseParsed.Path = strings.TrimRight(baseParsed.Path, "/") + "/api/services"
	q := baseParsed.Query()
	q.Set("limit", strconv.Itoa(limit))
	baseParsed.RawQuery = q.Encode()

	var resp models.ServicesResponse
	if err := c.doJSONGet(ctx, baseParsed.String(), apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CoremetryClient) GetServiceNames(ctx context.Context, baseURL, apiToken, query string, limit int) (*models.ServiceNamesResponse, error) {
	baseParsed, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	baseParsed.Path = strings.TrimRight(baseParsed.Path, "/") + "/api/service-names"
	q := baseParsed.Query()
	q.Set("limit", strconv.Itoa(limit))
	if strings.TrimSpace(query) != "" {
		q.Set("q", strings.TrimSpace(query))
	}
	baseParsed.RawQuery = q.Encode()

	var resp models.ServiceNamesResponse
	if err := c.doJSONGet(ctx, baseParsed.String(), apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CoremetryClient) GetOperationNames(ctx context.Context, baseURL, apiToken, service string, limit int) (*models.OperationNamesResponse, error) {
	baseParsed, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	baseParsed.Path = strings.TrimRight(baseParsed.Path, "/") + "/api/operation-names"
	q := baseParsed.Query()
	q.Set("limit", strconv.Itoa(limit))
	if strings.TrimSpace(service) != "" {
		q.Set("service", strings.TrimSpace(service))
	}
	baseParsed.RawQuery = q.Encode()

	var resp models.OperationNamesResponse
	if err := c.doJSONGet(ctx, baseParsed.String(), apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CoremetryClient) GetServiceMetricRED(
	ctx context.Context,
	baseURL, apiToken, service string,
	from, to time.Time,
	maxDP, rateWin int,
) (*models.ServiceREDResponse, error) {
	baseParsed, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(service) == "" {
		return nil, errors.New("service name is required for RED metrics")
	}

	encodedSvc := url.PathEscape(strings.TrimSpace(service))
	baseParsed.Path = strings.TrimRight(baseParsed.Path, "/") + "/api/services/" + encodedSvc + "/metric-red"
	q := baseParsed.Query()

	if !from.IsZero() {
		q.Set("from", strconv.FormatInt(from.UnixNano(), 10))
	}
	if !to.IsZero() {
		q.Set("to", strconv.FormatInt(to.UnixNano(), 10))
	}
	if maxDP > 0 {
		if maxDP > 4000 {
			maxDP = 4000
		}
		q.Set("maxDataPoints", strconv.Itoa(maxDP))
	}
	if rateWin > 0 {
		if rateWin > 600 {
			rateWin = 600
		}
		q.Set("rateWindow", strconv.Itoa(rateWin))
	}
	baseParsed.RawQuery = q.Encode()

	var resp models.ServiceREDResponse
	if err := c.doJSONGet(ctx, baseParsed.String(), apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CoremetryClient) GetLogs(ctx context.Context, baseURL, apiToken string, f models.LogFilter) (*models.LogsResponse, error) {
	baseParsed, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}

	baseParsed.Path = strings.TrimRight(baseParsed.Path, "/") + "/api/logs"
	q := baseParsed.Query()
	if strings.TrimSpace(f.Service) != "" {
		q.Set("service", strings.TrimSpace(f.Service))
	}
	if strings.TrimSpace(f.Search) != "" {
		q.Set("search", strings.TrimSpace(f.Search))
	}
	if strings.TrimSpace(f.Severity) != "" {
		q.Set("severity", strings.TrimSpace(f.Severity))
	}
	if strings.TrimSpace(f.TraceID) != "" {
		q.Set("traceId", strings.TrimSpace(f.TraceID))
	}
	if strings.TrimSpace(f.SpanID) != "" {
		q.Set("spanId", strings.TrimSpace(f.SpanID))
	}
	if !f.From.IsZero() {
		q.Set("from", strconv.FormatInt(f.From.UnixNano(), 10))
	}
	if !f.To.IsZero() {
		q.Set("to", strconv.FormatInt(f.To.UnixNano(), 10))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	q.Set("limit", strconv.Itoa(limit))
	baseParsed.RawQuery = q.Encode()

	var resp models.LogsResponse
	if err := c.doJSONGet(ctx, baseParsed.String(), apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CoremetryClient) GetTraces(ctx context.Context, baseURL, apiToken string, f models.TraceFilter) (*models.TracesResponse, error) {
	baseParsed, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}

	baseParsed.Path = strings.TrimRight(baseParsed.Path, "/") + "/api/traces"
	q := baseParsed.Query()
	if strings.TrimSpace(f.Service) != "" {
		q.Set("service", strings.TrimSpace(f.Service))
	}
	if strings.TrimSpace(f.TraceID) != "" {
		q.Set("traceId", strings.TrimSpace(f.TraceID))
	}
	if strings.TrimSpace(f.Search) != "" {
		q.Set("search", strings.TrimSpace(f.Search))
	}
	if !f.From.IsZero() {
		q.Set("from", strconv.FormatInt(f.From.UnixNano(), 10))
	}
	if !f.To.IsZero() {
		q.Set("to", strconv.FormatInt(f.To.UnixNano(), 10))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	q.Set("limit", strconv.Itoa(limit))
	baseParsed.RawQuery = q.Encode()

	var resp models.TracesResponse
	if err := c.doJSONGet(ctx, baseParsed.String(), apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func parseBaseURL(base string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return nil, errors.New("invalid Coremetry base URL")
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("Coremetry URL must be an absolute HTTP or HTTPS base URL")
	}
	return parsed, nil
}

func buildPromQLURL(base, query string, fromNs, toNs int64, stepSec, maxDP int) (string, error) {
	parsed, err := parseBaseURL(base)
	if err != nil {
		return "", err
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/metrics/promql"
	parsed.RawPath = ""

	q := parsed.Query()
	q.Set("query", query)
	if fromNs > 0 {
		q.Set("from", strconv.FormatInt(fromNs, 10))
	}
	if toNs > 0 {
		q.Set("to", strconv.FormatInt(toNs, 10))
	}
	if stepSec > 0 {
		q.Set("step", strconv.Itoa(stepSec))
	}
	if maxDP > 0 {
		if maxDP > 4000 {
			maxDP = 4000
		}
		q.Set("maxDataPoints", strconv.Itoa(maxDP))
	}
	q.Set("meta", "1")

	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

func parseAPIError(statusCode int, bodyBytes []byte) error {
	var errResp models.APIErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error != "" {
		return fmt.Errorf("%s", errResp.Error)
	}

	switch statusCode {
	case http.StatusBadRequest:
		return errors.New("invalid request syntax or parameters (HTTP 400)")
	case http.StatusUnauthorized:
		return errors.New("Coremetry rejected the API token (HTTP 401)")
	case http.StatusForbidden:
		return errors.New("the API token lacks viewer access (HTTP 403)")
	case http.StatusBadGateway:
		return errors.New("upstream backend error (HTTP 502)")
	default:
		return fmt.Errorf("Coremetry returned HTTP %d: %s", statusCode, string(bodyBytes))
	}
}

func calculateStep(from, to time.Time, userStep string, interval time.Duration, maxDataPoints int64) int {
	if strings.TrimSpace(userStep) != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(userStep)); err == nil && d > 0 {
			return int(d.Seconds())
		}
	}
	if interval > 0 {
		return int(interval.Seconds())
	}
	if maxDataPoints <= 0 {
		maxDataPoints = 300
	}
	diff := to.Sub(from)
	step := int(diff.Seconds()) / int(maxDataPoints)
	if step < 1 {
		step = 1
	}
	return step
}
