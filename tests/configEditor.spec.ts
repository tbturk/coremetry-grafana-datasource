import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render Coremetry datasource settings', async ({ createDataSourceConfigPage, page }) => {
  await createDataSourceConfigPage({ type: 'coremetry-datasource' });
  await expect(page.getByLabel('Coremetry URL')).toBeVisible();
  await expect(page.getByLabel('Viewer API token')).toBeVisible();
});

test('Save & Test reports when a viewer token is missing', async ({ createDataSourceConfigPage, page }) => {
  const configPage = await createDataSourceConfigPage({ type: 'coremetry-datasource' });
  await page.getByRole('textbox', { name: 'Coremetry URL' }).fill('http://coremetry:8088');
  await configPage.saveAndTest();
  await expect(configPage).toHaveAlert('error', { hasText: 'token is not configured' });
});
