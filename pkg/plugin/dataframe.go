package plugin

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cosretr/coremetry-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

var byRegex = regexp.MustCompile(`(?i)\bby\s*\(([^)]+)\)`)

func extractByLabels(query string) []string {
	match := byRegex.FindStringSubmatch(query)
	if len(match) < 2 {
		return nil
	}
	raw := match[1]
	parts := strings.Split(raw, ",")
	var labels []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			labels = append(labels, trimmed)
		}
	}
	return labels
}

func parseLabels(groupKey []string, byLabels []string) data.Labels {
	labels := make(data.Labels)
	for i, item := range groupKey {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if k != "" {
				labels[k] = v
				continue
			}
		}
		// Bare value
		if i < len(byLabels) {
			labels[byLabels[i]] = item
		} else if len(groupKey) == 1 {
			labels["group"] = item
		} else {
			labels[fmt.Sprintf("group_%d", i)] = item
		}
	}
	return labels
}

func buildSeriesName(labels data.Labels, defaultName string) string {
	if len(labels) == 0 {
		return defaultName
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%q", k, v))
	}
	sort.Strings(parts)
	return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
}

func SeriesToRangeFrames(refID string, query string, series []models.SpanMetricSeries, meta *models.SpanMetricMeta) data.Frames {
	if len(series) == 0 {
		emptyFrame := data.NewFrame(refID,
			data.NewField("time", nil, []time.Time{}),
			data.NewField("value", nil, []float64{}),
		)
		emptyFrame.RefID = refID
		emptyFrame.Meta = &data.FrameMeta{
			Type: data.FrameTypeTimeSeriesMulti,
		}
		return data.Frames{emptyFrame}
	}

	byLabels := extractByLabels(query)
	frames := make(data.Frames, 0, len(series))

	for i, s := range series {
		times := make([]time.Time, len(s.Points))
		values := make([]float64, len(s.Points))

		for j, pt := range s.Points {
			times[j] = time.Unix(0, pt.Time).UTC()
			values[j] = pt.Value
		}

		labels := parseLabels(s.GroupKey, byLabels)
		seriesName := buildSeriesName(labels, query)

		timeField := data.NewField("time", nil, times)
		valField := data.NewField("value", labels, values)
		valField.Config = &data.FieldConfig{
			DisplayNameFromDS: seriesName,
		}

		frameName := fmt.Sprintf("%s-%d", refID, i)
		frame := data.NewFrame(frameName, timeField, valField)
		frame.RefID = refID
		frame.Meta = &data.FrameMeta{
			Type: data.FrameTypeTimeSeriesMulti,
		}
		if meta != nil && meta.Executed != "" {
			frame.Meta.ExecutedQueryString = meta.Executed
		}

		frames = append(frames, frame)
	}

	return frames
}

func SeriesToInstantFrames(refID string, query string, targetTime time.Time, series []models.SpanMetricSeries, meta *models.SpanMetricMeta) data.Frames {
	if len(series) == 0 {
		emptyFrame := data.NewFrame(refID,
			data.NewField("time", nil, []time.Time{}),
			data.NewField("value", nil, []float64{}),
		)
		emptyFrame.RefID = refID
		emptyFrame.Meta = &data.FrameMeta{
			Type: data.FrameTypeTimeSeriesMulti,
		}
		return data.Frames{emptyFrame}
	}

	byLabels := extractByLabels(query)
	frames := make(data.Frames, 0, len(series))

	for i, s := range series {
		if len(s.Points) == 0 {
			continue
		}
		lastPt := s.Points[len(s.Points)-1]
		ptTime := time.Unix(0, lastPt.Time).UTC()
		if ptTime.IsZero() {
			ptTime = targetTime
		}

		labels := parseLabels(s.GroupKey, byLabels)
		seriesName := buildSeriesName(labels, query)

		timeField := data.NewField("time", nil, []time.Time{ptTime})
		valField := data.NewField("value", labels, []float64{lastPt.Value})
		valField.Config = &data.FieldConfig{
			DisplayNameFromDS: seriesName,
		}

		frameName := fmt.Sprintf("%s-%d", refID, i)
		frame := data.NewFrame(frameName, timeField, valField)
		frame.RefID = refID
		frame.Meta = &data.FrameMeta{
			Type: data.FrameTypeTimeSeriesMulti,
		}
		if meta != nil && meta.Executed != "" {
			frame.Meta.ExecutedQueryString = meta.Executed
		}

		frames = append(frames, frame)
	}

	if len(frames) == 0 {
		emptyFrame := data.NewFrame(refID,
			data.NewField("time", nil, []time.Time{}),
			data.NewField("value", nil, []float64{}),
		)
		emptyFrame.RefID = refID
		emptyFrame.Meta = &data.FrameMeta{
			Type: data.FrameTypeTimeSeriesMulti,
		}
		return data.Frames{emptyFrame}
	}

	return frames
}

func ServicesToDataFrame(services []models.ServiceSummary, refID string) *data.Frame {
	names := make([]string, len(services))
	healths := make([]string, len(services))
	spanCounts := make([]int64, len(services))
	errorCounts := make([]int64, len(services))
	errorRates := make([]float64, len(services))
	avgDurations := make([]float64, len(services))
	p99Durations := make([]float64, len(services))
	apdexes := make([]float64, len(services))
	lastSeens := make([]time.Time, len(services))

	for i, s := range services {
		names[i] = s.Name
		healths[i] = s.Health
		spanCounts[i] = s.SpanCount
		errorCounts[i] = s.ErrorCount
		errorRates[i] = s.ErrorRate
		avgDurations[i] = s.AvgDurationMs
		p99Durations[i] = s.P99DurationMs
		apdexes[i] = s.Apdex
		if s.LastSeen > 0 {
			lastSeens[i] = time.Unix(0, s.LastSeen).UTC()
		}
	}

	errRateField := data.NewField("error_rate", nil, errorRates)
	errRateField.Config = &data.FieldConfig{Unit: "percent"}

	avgDurField := data.NewField("avg_duration_ms", nil, avgDurations)
	avgDurField.Config = &data.FieldConfig{Unit: "ms"}

	p99DurField := data.NewField("p99_duration_ms", nil, p99Durations)
	p99DurField.Config = &data.FieldConfig{Unit: "ms"}

	frame := data.NewFrame(
		refID,
		data.NewField("name", nil, names),
		data.NewField("health", nil, healths),
		data.NewField("span_count", nil, spanCounts),
		data.NewField("error_count", nil, errorCounts),
		errRateField,
		avgDurField,
		p99DurField,
		data.NewField("apdex", nil, apdexes),
		data.NewField("last_seen", nil, lastSeens),
	)
	frame.RefID = refID
	return frame
}

func REDResponseToDataFrames(red *models.ServiceREDResponse, selectedMetric string, refID string) data.Frames {
	if red == nil || len(red.Series) == 0 {
		emptyFrame := data.NewFrame(refID,
			data.NewField("time", nil, []time.Time{}),
			data.NewField("value", nil, []float64{}),
		)
		emptyFrame.RefID = refID
		emptyFrame.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
		return data.Frames{emptyFrame}
	}

	selected := strings.ToLower(strings.TrimSpace(selectedMetric))
	if selected == "" {
		selected = "all"
	}

	metricsToInclude := []string{"rate", "error_rate", "avg", "p50", "p95", "p99"}
	if selected != "all" {
		metricsToInclude = []string{selected}
	}

	frames := make(data.Frames, 0)
	for _, mKey := range metricsToInclude {
		seriesList, ok := red.Series[mKey]
		if !ok || len(seriesList) == 0 {
			continue
		}

		for idx, s := range seriesList {
			times := make([]time.Time, len(s.Points))
			values := make([]float64, len(s.Points))
			for j, pt := range s.Points {
				times[j] = time.Unix(0, pt.Time).UTC()
				values[j] = pt.Value
			}

			displayName := fmt.Sprintf("%s (%s)", red.Service, mKey)
			if len(s.GroupKey) > 0 {
				displayName = fmt.Sprintf("%s (%s - %s)", red.Service, mKey, strings.Join(s.GroupKey, ", "))
			}

			labels := data.Labels{
				"service": red.Service,
				"metric":  mKey,
			}
			for k, v := range parseLabels(s.GroupKey, nil) {
				labels[k] = v
			}

			timeField := data.NewField("time", nil, times)
			valField := data.NewField("value", labels, values)

			unit := "short"
			switch mKey {
			case "rate":
				unit = "reqps"
			case "error_rate":
				unit = "percent"
			case "avg", "p50", "p95", "p99":
				unit = "ms"
			}
			valField.Config = &data.FieldConfig{
				DisplayNameFromDS: displayName,
				Unit:              unit,
			}

			fName := fmt.Sprintf("%s-%s-%d", refID, mKey, idx)
			f := data.NewFrame(fName, timeField, valField)
			f.RefID = refID
			f.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
			frames = append(frames, f)
		}
	}

	if len(frames) == 0 {
		emptyFrame := data.NewFrame(refID,
			data.NewField("time", nil, []time.Time{}),
			data.NewField("value", nil, []float64{}),
		)
		emptyFrame.RefID = refID
		emptyFrame.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
		return data.Frames{emptyFrame}
	}

	return frames
}

func LogsToDataFrame(logs []*models.LogRecord, refID string, traceDatasourceUID string) *data.Frame {
	if traceDatasourceUID == "" {
		traceDatasourceUID = "coremetry-tempo"
	}

	times := make([]time.Time, len(logs))
	lines := make([]string, len(logs))
	levels := make([]string, len(logs))
	services := make([]string, len(logs))
	traceIDs := make([]string, len(logs))
	spanIDs := make([]string, len(logs))

	for i, l := range logs {
		if l.Timestamp > 0 {
			times[i] = time.Unix(0, l.Timestamp).UTC()
		}
		lines[i] = l.Body
		levels[i] = l.SeverityText
		services[i] = l.ServiceName
		traceIDs[i] = l.TraceID
		spanIDs[i] = l.SpanID
	}

	traceField := data.NewField("traceId", nil, traceIDs)
	if traceDatasourceUID != "" {
		traceField.Config = &data.FieldConfig{
			Links: []data.DataLink{
				{
					Title:       "View trace in Tempo",
					URL:         "/explore?left={\"datasource\":\"" + traceDatasourceUID + "\",\"queries\":[{\"query\":\"${__value.raw}\",\"queryType\":\"traceql\"}]}",
					TargetBlank: false,
				},
			},
		}
	}

	frame := data.NewFrame(
		refID,
		data.NewField("time", nil, times),
		data.NewField("content", nil, lines),
		data.NewField("level", nil, levels),
		data.NewField("service", nil, services),
		traceField,
		data.NewField("spanId", nil, spanIDs),
	)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{
		PreferredVisualization: data.VisTypeLogs,
	}
	return frame
}

func TracesToDataFrame(traces []models.TraceSummary, refID string, traceDatasourceUID string) *data.Frame {
	if traceDatasourceUID == "" {
		traceDatasourceUID = "coremetry-tempo"
	}

	times := make([]time.Time, len(traces))
	traceIDs := make([]string, len(traces))
	services := make([]string, len(traces))
	rootNames := make([]string, len(traces))
	durations := make([]float64, len(traces))
	spanCounts := make([]int64, len(traces))
	hasErrors := make([]bool, len(traces))

	for i, t := range traces {
		if t.StartTime > 0 {
			times[i] = time.Unix(0, t.StartTime).UTC()
		}
		traceIDs[i] = t.TraceID
		services[i] = t.ServiceName
		rootNames[i] = t.RootName
		durations[i] = t.DurationMs
		spanCounts[i] = int64(t.SpanCount)
		hasErrors[i] = t.HasError
	}

	traceField := data.NewField("traceId", nil, traceIDs)
	if traceDatasourceUID != "" {
		traceField.Config = &data.FieldConfig{
			Links: []data.DataLink{
				{
					Title:       "View trace in Tempo",
					URL:         "/explore?left={\"datasource\":\"" + traceDatasourceUID + "\",\"queries\":[{\"query\":\"${__value.raw}\",\"queryType\":\"traceql\"}]}",
					TargetBlank: false,
				},
			},
		}
	}

	durField := data.NewField("duration_ms", nil, durations)
	durField.Config = &data.FieldConfig{Unit: "ms"}

	frame := data.NewFrame(
		refID,
		data.NewField("time", nil, times),
		traceField,
		data.NewField("service", nil, services),
		data.NewField("root_name", nil, rootNames),
		durField,
		data.NewField("span_count", nil, spanCounts),
		data.NewField("has_error", nil, hasErrors),
	)
	frame.RefID = refID
	return frame
}
