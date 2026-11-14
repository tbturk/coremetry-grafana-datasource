import React, { ChangeEvent } from 'react';
import { QueryEditorProps } from '@grafana/data';
import { InlineField, InlineFieldRow, Input, RadioButtonGroup, Select, TextArea } from '@grafana/ui';
import { DataSource } from '../datasource';
import { CoremetryDataSourceOptions, CoremetryQuery, CoremetryQueryType, DEFAULT_QUERY } from '../types';

type Props = QueryEditorProps<DataSource, CoremetryQuery, CoremetryDataSourceOptions>;

const queryTypeOptions: Array<{ label: string; value: CoremetryQueryType; description: string }> = [
  { label: 'Metrics (PromQL)', value: 'metrics', description: 'PromQL queries over Coremetry metric points' },
  { label: 'Services', value: 'services', description: 'Service catalogue and health overview' },
  { label: 'Service RED', value: 'red', description: 'Service-level Rate, Error rate, and Duration metrics' },
  { label: 'Logs', value: 'logs', description: 'Log querying with service, trace_id, and severity filters' },
  { label: 'Traces', value: 'traces', description: 'Trace search and navigation' },
];

const metricRangeOptions = [
  { label: 'Range', value: 'range', description: 'Evaluate expression over the time range' },
  { label: 'Instant', value: 'instant', description: 'Evaluate expression at the end time point' },
];

const redMetricOptions = [
  { label: 'All RED Metrics', value: 'all' },
  { label: 'Request Rate (req/s)', value: 'rate' },
  { label: 'Error Rate (%)', value: 'error_rate' },
  { label: 'Average Latency (ms)', value: 'avg' },
  { label: 'P50 Latency (ms)', value: 'p50' },
  { label: 'P95 Latency (ms)', value: 'p95' },
  { label: 'P99 Latency (ms)', value: 'p99' },
];

const severityOptions = [
  { label: 'All Severities', value: '' },
  { label: 'DEBUG (5+)', value: '5' },
  { label: 'INFO (9+)', value: '9' },
  { label: 'WARN (13+)', value: '13' },
  { label: 'ERROR (17+)', value: '17' },
  { label: 'FATAL (21+)', value: '21' },
];

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const currentQuery: CoremetryQuery = {
    ...DEFAULT_QUERY,
    ...query,
  };

  const currentQueryType: CoremetryQueryType = currentQuery.queryType ?? 'metrics';

  const onQueryTypeChange = (value: CoremetryQueryType) => {
    onChange({
      ...currentQuery,
      queryType: value,
    });
    onRunQuery();
  };

  const onRawQueryChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    onChange({ ...currentQuery, rawQuery: e.target.value });
  };

  const onStepChange = (e: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...currentQuery, step: e.target.value });
    onRunQuery();
  };

  const onMetricTypeChange = (value: string) => {
    const isInstant = value === 'instant';
    onChange({
      ...currentQuery,
      instant: isInstant,
      range: !isInstant,
    });
    onRunQuery();
  };

  const onServiceChange = (e: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...currentQuery, service: e.target.value });
  };

  const onREDMetricChange = (opt: { value?: string }) => {
    onChange({ ...currentQuery, redMetric: opt.value ?? 'all' });
    onRunQuery();
  };

  const onSearchChange = (e: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...currentQuery, search: e.target.value });
  };

  const onSeverityChange = (opt: { value?: string }) => {
    onChange({ ...currentQuery, severity: opt.value ?? '' });
    onRunQuery();
  };

  const onTraceIdChange = (e: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...currentQuery, traceId: e.target.value });
  };

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const limitNum = parseInt(e.target.value, 10);
    onChange({ ...currentQuery, limit: isNaN(limitNum) ? undefined : limitNum });
  };

  const selectedMetricType = currentQuery.instant ? 'instant' : 'range';

  return (
    <div className="gf-form-group">
      <InlineFieldRow>
        <InlineField label="Query Mode" labelWidth={14} tooltip="Select signal or view type">
          <RadioButtonGroup options={queryTypeOptions} value={currentQueryType} onChange={onQueryTypeChange} />
        </InlineField>
      </InlineFieldRow>

      {/* Mode: Metrics (PromQL) */}
      {currentQueryType === 'metrics' && (
        <>
          <InlineFieldRow>
            <InlineField
              label="Expression"
              labelWidth={14}
              grow
              tooltip="Enter a PromQL expression (e.g. http.server.requests, sum by(service_name)(rate(http.server.requests[5m])))"
            >
              <TextArea
                id={`query-editor-expression-${query.refId}`}
                aria-label="PromQL Expression"
                value={currentQuery.rawQuery ?? ''}
                placeholder="e.g. sum by(service_name) (rate(http.server.requests[5m]))"
                rows={3}
                onChange={onRawQueryChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Type"
              labelWidth={14}
              tooltip="Select range vector (graph) or instant vector (table/stat)"
            >
              <RadioButtonGroup options={metricRangeOptions} value={selectedMetricType} onChange={onMetricTypeChange} />
            </InlineField>

            <InlineField
              label="Min step"
              labelWidth={14}
              tooltip="Resolution step interval (e.g. 15s, 1m). Leave empty for auto resolution."
            >
              <Input
                id={`query-editor-step-${query.refId}`}
                aria-label="Min step"
                value={currentQuery.step ?? ''}
                placeholder="auto"
                width={16}
                onChange={onStepChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>
        </>
      )}

      {/* Mode: Services */}
      {currentQueryType === 'services' && (
        <InlineFieldRow>
          <InlineField label="Max Services" labelWidth={14} tooltip="Maximum number of services to return (max 500)">
            <Input
              id={`query-editor-limit-${query.refId}`}
              aria-label="Max Services"
              type="number"
              value={currentQuery.limit ?? 50}
              placeholder="50"
              width={16}
              onChange={onLimitChange}
              onBlur={onRunQuery}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {/* Mode: Service RED */}
      {currentQueryType === 'red' && (
        <InlineFieldRow>
          <InlineField
            label="Service"
            labelWidth={14}
            tooltip="Name of the service (e.g. coremetry-monolithic, openbanking-gateway, $service)"
          >
            <Input
              id={`query-editor-service-${query.refId}`}
              aria-label="Service Name"
              value={currentQuery.service ?? ''}
              placeholder="e.g. coremetry-monolithic or $service"
              width={30}
              onChange={onServiceChange}
              onBlur={onRunQuery}
            />
          </InlineField>

          <InlineField label="RED Metric" labelWidth={14} tooltip="Select specific RED metric or All">
            <Select
              aria-label="RED Metric"
              options={redMetricOptions}
              value={currentQuery.redMetric ?? 'all'}
              onChange={onREDMetricChange}
              width={26}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {/* Mode: Logs */}
      {currentQueryType === 'logs' && (
        <>
          <InlineFieldRow>
            <InlineField
              label="Service"
              labelWidth={14}
              tooltip="Filter logs by service name (e.g. auth-service or $service)"
            >
              <Input
                id={`query-editor-log-service-${query.refId}`}
                aria-label="Service Filter"
                value={currentQuery.service ?? ''}
                placeholder="All services"
                width={24}
                onChange={onServiceChange}
                onBlur={onRunQuery}
              />
            </InlineField>

            <InlineField label="Severity" labelWidth={14} tooltip="Minimum log severity level">
              <Select
                aria-label="Severity Filter"
                options={severityOptions}
                value={currentQuery.severity ?? ''}
                onChange={onSeverityChange}
                width={20}
              />
            </InlineField>

            <InlineField label="Limit" labelWidth={10} tooltip="Maximum log lines to return (max 1000)">
              <Input
                id={`query-editor-log-limit-${query.refId}`}
                aria-label="Log Limit"
                type="number"
                value={currentQuery.limit ?? 100}
                placeholder="100"
                width={12}
                onChange={onLimitChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Search" labelWidth={14} grow tooltip="Full text / substring search filter">
              <Input
                id={`query-editor-log-search-${query.refId}`}
                aria-label="Search Filter"
                value={currentQuery.search ?? ''}
                placeholder="e.g. error, failed, exception"
                onChange={onSearchChange}
                onBlur={onRunQuery}
              />
            </InlineField>

            <InlineField
              label="Trace ID"
              labelWidth={14}
              tooltip="Filter logs associated with a specific trace ID (correlation)"
            >
              <Input
                id={`query-editor-log-traceid-${query.refId}`}
                aria-label="Trace ID Filter"
                value={currentQuery.traceId ?? ''}
                placeholder="Filter by traceId"
                width={32}
                onChange={onTraceIdChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>
        </>
      )}

      {/* Mode: Traces */}
      {currentQueryType === 'traces' && (
        <InlineFieldRow>
          <InlineField label="Service" labelWidth={14} tooltip="Filter traces by service name">
            <Input
              id={`query-editor-trace-service-${query.refId}`}
              aria-label="Service Filter"
              value={currentQuery.service ?? ''}
              placeholder="All services"
              width={24}
              onChange={onServiceChange}
              onBlur={onRunQuery}
            />
          </InlineField>

          <InlineField label="Trace ID" labelWidth={14} tooltip="Filter by exact trace ID">
            <Input
              id={`query-editor-trace-id-${query.refId}`}
              aria-label="Trace ID Filter"
              value={currentQuery.traceId ?? ''}
              placeholder="Exact traceId"
              width={32}
              onChange={onTraceIdChange}
              onBlur={onRunQuery}
            />
          </InlineField>

          <InlineField label="Limit" labelWidth={10} tooltip="Maximum traces to return">
            <Input
              id={`query-editor-trace-limit-${query.refId}`}
              aria-label="Trace Limit"
              type="number"
              value={currentQuery.limit ?? 50}
              placeholder="50"
              width={12}
              onChange={onLimitChange}
              onBlur={onRunQuery}
            />
          </InlineField>
        </InlineFieldRow>
      )}
    </div>
  );
}
