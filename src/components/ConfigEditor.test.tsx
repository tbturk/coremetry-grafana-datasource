import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { ConfigEditor } from './ConfigEditor';
import { CoremetryDataSourceOptions, CoremetrySecureJsonData } from '../types';

type Props = DataSourcePluginOptionsEditorProps<CoremetryDataSourceOptions, CoremetrySecureJsonData>;
type TestOptions = Omit<Props['options'], 'database'> & { database: string };

function createOptions(): TestOptions {
  return {
    id: 1,
    uid: 'coremetry',
    orgId: 1,
    name: 'Coremetry',
    typeLogoUrl: '',
    type: 'coremetry-datasource',
    typeName: 'Coremetry',
    access: 'proxy',
    url: 'http://coremetry:8088',
    user: '',
    database: '',
    basicAuth: false,
    basicAuthUser: '',
    isDefault: false,
    jsonData: {},
    secureJsonData: {},
    secureJsonFields: {},
    readOnly: false,
    withCredentials: false,
  };
}

describe('ConfigEditor', () => {
  it('stores the Coremetry URL in datasource options', () => {
    const onOptionsChange = jest.fn();
    const props = {
      options: createOptions(),
      onOptionsChange,
    } satisfies Props;

    render(<ConfigEditor {...props} />);
    fireEvent.change(screen.getByLabelText('Coremetry URL'), {
      target: { value: 'http://coremetry.internal:8088' },
    });

    expect(onOptionsChange).toHaveBeenCalledWith(expect.objectContaining({ url: 'http://coremetry.internal:8088' }));
  });

  it('uses Grafana secure datasource settings for the viewer token', () => {
    const onOptionsChange = jest.fn();
    const props = {
      options: createOptions(),
      onOptionsChange,
    } satisfies Props;

    render(<ConfigEditor {...props} />);
    const tokenInput = screen.getByLabelText('Viewer API token');
    expect(tokenInput).toHaveAttribute('type', 'password');

    fireEvent.change(tokenInput, { target: { value: 'viewer-token' } });
    expect(onOptionsChange).toHaveBeenCalledWith(
      expect.objectContaining({
        secureJsonData: { apiToken: 'viewer-token' },
      })
    );
  });

  it('configures trace datasource UID', () => {
    const onOptionsChange = jest.fn();
    const props = {
      options: createOptions(),
      onOptionsChange,
    } satisfies Props;

    render(<ConfigEditor {...props} />);
    const traceInput = screen.getByLabelText('Trace datasource UID');
    fireEvent.change(traceInput, { target: { value: 'tempo-custom-uid' } });

    expect(onOptionsChange).toHaveBeenCalledWith(
      expect.objectContaining({
        jsonData: { traceDatasourceUid: 'tempo-custom-uid' },
      })
    );
  });
});
