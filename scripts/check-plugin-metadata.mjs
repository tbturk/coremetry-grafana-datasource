import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const read = (path) => readFile(resolve(root, path), 'utf8');
const plugin = JSON.parse(await read('src/plugin.json'));
const pluginId = plugin.id;

if (pluginId !== 'coremetry-datasource') {
  throw new Error(`Unexpected plugin ID: ${JSON.stringify(pluginId)}`);
}
if (!/^[0-9a-z]+-([0-9a-z]+-)?(app|panel|datasource)$/.test(pluginId)) {
  throw new Error(`Plugin ID does not follow Grafana's ID convention: ${pluginId}`);
}
if (plugin.type !== 'datasource' || plugin.backend !== true) {
  throw new Error('plugin.json must describe a datasource with a Go backend');
}

const requiredReferences = [
  ['pkg/main.go', `Manage("${pluginId}"`],
  ['docker-compose.yaml', `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS: ${pluginId}`],
  ['provisioning/datasources/coremetry.yaml', `type: ${pluginId}`],
  ['.config/docker-compose-base.yaml', `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS: ${pluginId}`],
  ['.config/supervisord/supervisord.conf', `/var/lib/grafana/plugins/${pluginId}`],
];
for (const [path, expected] of requiredReferences) {
  if (!(await read(path)).includes(expected)) {
    throw new Error(`${path} does not match plugin ID ${JSON.stringify(pluginId)}`);
  }
}

const dashboard = JSON.parse(await read('provisioning/dashboards/files/coremetry-development.json'));
if (dashboard.uid !== 'coremetry-dev') {
  throw new Error("Development dashboard must have UID 'coremetry-dev'");
}

console.log(`Plugin metadata is consistent: ${pluginId}`);
