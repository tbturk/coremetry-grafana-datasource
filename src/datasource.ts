import { CoreApp } from '@grafana/data';
import { DataSourceWithBackend } from '@grafana/runtime';

import { CoremetryDataSourceOptions, CoremetryQuery } from './types';

export class DataSource extends DataSourceWithBackend<CoremetryQuery, CoremetryDataSourceOptions> {
  getDefaultQuery(_app: CoreApp): Partial<CoremetryQuery> {
    return { refId: 'A' };
  }

  filterQuery(_query: CoremetryQuery): boolean {
    return false;
  }
}
