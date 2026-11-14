import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type CoremetryQueryType = 'metrics' | 'services' | 'red' | 'logs' | 'traces';

export interface CoremetryQuery extends DataQuery {
  queryType?: CoremetryQueryType;
  rawQuery?: string;
  instant?: boolean;
  range?: boolean;
  step?: string;

  // APM fields
  service?: string;
  redMetric?: string; // all, rate, error_rate, p50, p95, p99, avg
  search?: string;
  severity?: string;
  traceId?: string;
  limit?: number;
}

export const DEFAULT_QUERY: Partial<CoremetryQuery> = {
  queryType: 'metrics',
  rawQuery: '',
  instant: false,
  range: true,
  step: '',
  service: '',
  redMetric: 'all',
  search: '',
  severity: '',
  traceId: '',
  limit: 100,
};

export interface CoremetryDataSourceOptions extends DataSourceJsonData {
  traceDatasourceUid?: string;
}

export interface CoremetrySecureJsonData {
  apiToken?: string;
}
