import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { QueryEditorProps } from '@grafana/data';
import { QueryEditor } from './QueryEditor';
import { DataSource } from '../datasource';
import { CoremetryDataSourceOptions, CoremetryQuery } from '../types';

type Props = QueryEditorProps<DataSource, CoremetryQuery, CoremetryDataSourceOptions>;

function createProps(queryOverrides?: Partial<CoremetryQuery>): Props {
  const query: CoremetryQuery = {
    refId: 'A',
    queryType: 'metrics',
    rawQuery: 'http.server.requests',
    range: true,
    instant: false,
    step: '15s',
    ...queryOverrides,
  };

  return {
    datasource: {} as DataSource,
    query,
    onChange: jest.fn(),
    onRunQuery: jest.fn(),
    onBlur: jest.fn(),
  };
}

describe('QueryEditor', () => {
  it('renders expression input and step input in metrics mode', () => {
    const props = createProps();
    render(<QueryEditor {...props} />);

    expect(screen.getByLabelText('PromQL Expression')).toHaveValue('http.server.requests');
    expect(screen.getByLabelText('Min step')).toHaveValue('15s');
    expect(screen.getByRole('radio', { name: 'Range' })).toBeChecked();
    expect(screen.getByRole('radio', { name: 'Instant' })).not.toBeChecked();
  });

  it('triggers onChange when expression changes and onRunQuery on blur', () => {
    const props = createProps();
    render(<QueryEditor {...props} />);

    const textarea = screen.getByLabelText('PromQL Expression');
    fireEvent.change(textarea, { target: { value: 'rate(http.server.requests[5m])' } });

    expect(props.onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        rawQuery: 'rate(http.server.requests[5m])',
      })
    );

    fireEvent.blur(textarea);
    expect(props.onRunQuery).toHaveBeenCalled();
  });

  it('toggles query type to instant', () => {
    const props = createProps();
    render(<QueryEditor {...props} />);

    const instantRadio = screen.getByRole('radio', { name: 'Instant' });
    fireEvent.click(instantRadio);

    expect(props.onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        instant: true,
        range: false,
      })
    );
    expect(props.onRunQuery).toHaveBeenCalled();
  });

  it('switches to services mode', () => {
    const props = createProps();
    render(<QueryEditor {...props} />);

    const servicesRadio = screen.getByRole('radio', { name: 'Services' });
    fireEvent.click(servicesRadio);

    expect(props.onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        queryType: 'services',
      })
    );
  });

  it('renders logs inputs in logs mode', () => {
    const props = createProps({ queryType: 'logs', service: 'auth-service', search: 'login' });
    render(<QueryEditor {...props} />);

    expect(screen.getByLabelText('Service Filter')).toHaveValue('auth-service');
    expect(screen.getByLabelText('Search Filter')).toHaveValue('login');
    expect(screen.getByLabelText('Trace ID Filter')).toBeInTheDocument();
  });

  it('renders red inputs in red mode', () => {
    const props = createProps({ queryType: 'red', service: 'api-gateway' });
    render(<QueryEditor {...props} />);

    expect(screen.getByLabelText('Service Name')).toHaveValue('api-gateway');
    expect(screen.getByLabelText('RED Metric')).toBeInTheDocument();
  });

  it('renders traces inputs in traces mode', () => {
    const props = createProps({ queryType: 'traces', traceId: 'trace-123' });
    render(<QueryEditor {...props} />);

    expect(screen.getByLabelText('Trace ID Filter')).toHaveValue('trace-123');
  });
});
