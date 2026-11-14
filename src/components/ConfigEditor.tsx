import React, { ChangeEvent } from 'react';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { InlineField, Input, SecretInput } from '@grafana/ui';
import { CoremetryDataSourceOptions, CoremetrySecureJsonData } from '../types';

type Props = DataSourcePluginOptionsEditorProps<CoremetryDataSourceOptions, CoremetrySecureJsonData>;

export function ConfigEditor({ onOptionsChange, options }: Props) {
  const { secureJsonData, jsonData } = options;
  const secureJsonFields = options.secureJsonFields ?? {};

  const onUrlChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      url: event.target.value,
    });
  };

  const onTraceDatasourceUidChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        traceDatasourceUid: event.target.value,
      },
    });
  };

  const onApiTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        apiToken: event.target.value,
      },
    });
  };

  const onResetApiToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...secureJsonFields,
        apiToken: false,
      },
      secureJsonData: {
        ...secureJsonData,
        apiToken: '',
      },
    });
  };

  return (
    <>
      <InlineField
        label="Coremetry URL"
        labelWidth={20}
        interactive
        tooltip="Base URL of the Coremetry API, without an /api suffix."
      >
        <Input
          id="config-editor-coremetry-url"
          aria-label="Coremetry URL"
          onChange={onUrlChange}
          value={options.url ?? ''}
          placeholder="http://coremetry:8088"
          width={40}
        />
      </InlineField>
      <InlineField
        label="Viewer API token"
        labelWidth={20}
        interactive
        tooltip="Stored as a secure datasource field and used only by the Go backend."
      >
        <SecretInput
          id="config-editor-api-token"
          aria-label="Viewer API token"
          isConfigured={Boolean(secureJsonFields.apiToken)}
          value={secureJsonData?.apiToken}
          placeholder="Enter a Coremetry viewer token"
          width={40}
          onReset={onResetApiToken}
          onChange={onApiTokenChange}
        />
      </InlineField>
      <InlineField
        label="Trace datasource UID"
        labelWidth={20}
        interactive
        tooltip="UID of the Grafana Tempo datasource connected to Coremetry (/tempo). Used for trace correlation links."
      >
        <Input
          id="config-editor-trace-ds-uid"
          aria-label="Trace datasource UID"
          onChange={onTraceDatasourceUidChange}
          value={jsonData?.traceDatasourceUid ?? ''}
          placeholder="coremetry-tempo"
          width={40}
        />
      </InlineField>
    </>
  );
}
