import React from 'react';
import { Alert } from '@grafana/ui';

export function QueryEditor() {
  return (
    <Alert title="Metric queries are not available yet" severity="info">
      Query execution is intentionally disabled in Phase 2. Metric query support is planned for Phase 3.
    </Alert>
  );
}
