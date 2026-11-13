import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type CoremetryQuery = DataQuery;

export type CoremetryDataSourceOptions = DataSourceJsonData;

export interface CoremetrySecureJsonData {
  apiToken?: string;
}
