import { DataSourcePlugin } from '@grafana/data';
import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { CoremetryQuery, CoremetryDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, CoremetryQuery, CoremetryDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);
