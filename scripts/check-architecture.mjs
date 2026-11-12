import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const server = fileURLToPath(new URL('../server/', import.meta.url));
const modulePath = execFileSync('go', ['list', '-m'], { cwd: server, encoding: 'utf8' }).trim();
const prefix = `${modulePath}/internal/`;
const rules = {
  api: new Set(['app', 'domain', 'config']),
  app: new Set(['domain', 'config']),
  domain: new Set(),
  infra: new Set(['app', 'domain', 'config']),
  bootstrap: new Set(['api', 'app', 'domain', 'infra', 'config']),
  testutil: new Set(['app', 'domain', 'infra', 'config']),
  config: new Set(),
};
const output = execFileSync('go', ['list', '-f', '{{.ImportPath}}|{{join .Imports ","}}', './internal/...'], {
  cwd: server, encoding: 'utf8',
});
const failures = [];
const found = new Set();
const layerOf = (packagePath) => packagePath.slice(prefix.length).split('/')[0];
for (const row of output.trim().split('\n')) {
  const [packagePath, importList = ''] = row.split('|');
  const layer = layerOf(packagePath);
  if (!Object.hasOwn(rules, layer)) {
    failures.push(`Unregistered internal package: ${packagePath}`);
    continue;
  }
  found.add(layer);
  for (const dependency of importList.split(',').filter(Boolean)) {
    if (dependency.startsWith(prefix) && !rules[layer].has(layerOf(dependency))) {
      failures.push(`${layer} must not import ${dependency}`);
    }
    if (layer === 'domain' && (
      dependency.startsWith(`${modulePath}/`) ||
      /^(net\/|database\/|xorm\.io\/|gorm\.io\/|github\.com\/(emersion|go-acme)\/)/.test(dependency)
    )) {
      failures.push(`Domain depends on transport, persistence, or a compatibility package: ${dependency}`);
    }
  }
}
for (const layer of Object.keys(rules)) {
  if (!found.has(layer)) failures.push(`Missing layer: ${layer}`);
}
if (failures.length) {
  process.stderr.write(`${failures.join('\n')}\n`);
  process.exitCode = 1;
} else {
  process.stdout.write('Architecture OK: inward dependencies, isolated domain.\n');
}
