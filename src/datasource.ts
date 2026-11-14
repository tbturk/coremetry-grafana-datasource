import { CoreApp, MetricFindValue } from '@grafana/data';
import { DataSourceWithBackend } from '@grafana/runtime';

import { CoremetryDataSourceOptions, CoremetryQuery, DEFAULT_QUERY } from './types';

export class DataSource extends DataSourceWithBackend<CoremetryQuery, CoremetryDataSourceOptions> {
  getDefaultQuery(_app: CoreApp): Partial<CoremetryQuery> {
    return {
      refId: 'A',
      ...DEFAULT_QUERY,
    };
  }

  filterQuery(query: CoremetryQuery): boolean {
    switch (query.queryType) {
      case 'services':
      case 'logs':
      case 'traces':
        return true;
      case 'red':
        return Boolean(query.service && query.service.trim() !== '');
      case 'metrics':
      default:
        return Boolean(query.rawQuery && query.rawQuery.trim() !== '');
    }
  }

  async metricFindQuery(query: string, _options?: unknown): Promise<MetricFindValue[]> {
    const trimmed = (query || '').trim();

    // Match operations(<service>) or endpoints(<service>)
    const opMatch = trimmed.match(/^(?:operations|endpoints)\(([^)]*)\)$/i);
    if (opMatch) {
      const service = opMatch[1].trim();
      try {
        const resp = await this.getResource<{ names?: string[] }>('operations', { service });
        const names = resp?.names || [];
        return names.map((name) => ({ text: name, value: name }));
      } catch {
        return [];
      }
    }

    // Default / "services" / "services()" / empty
    if (!trimmed || trimmed.toLowerCase().startsWith('services')) {
      try {
        const resp = await this.getResource<{ names?: string[] }>('services');
        const names = resp?.names || [];
        return names.map((name) => ({ text: name, value: name }));
      } catch {
        return [];
      }
    }

    return [];
  }
}
