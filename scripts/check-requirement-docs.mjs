#!/usr/bin/env node
// 需求交付文档五件套与索引校验：命名、H1、头部链接、表格、图片；流程图由 project-docs-bootstrap 脚本按功能点校验。
// 用法：node check-docs.mjs --docs <仓库>/docs/需求 --issue <需求号> [--skip-index]
import fs from 'node:fs';
import path from 'node:path';

const TYPES = ['需求清单', '方案评估', '实施方案', '方案验收', '前端对接文档'];
const LINKED_TYPES = ['方案评估', '实施方案', '方案验收']; // 这三份必须链接需求清单
const STATUS_FORBIDDEN = ['已完成'];
const INDEX_CANDIDATES = ['README.md', 'index.md', '00-索引.md', '文档索引.md'];

function parseArgs(argv) {
  const args = {};
  for (let i = 2; i < argv.length; i++) {
    const key = argv[i].replace(/^--/, '');
    if (key === 'skip-index') {
      args[key] = true;
      continue;
    }
    args[key] = argv[++i];
  }
  return args;
}

const args = parseArgs(process.argv);
const { docs, issue } = args;
const skipIndex = Boolean(args['skip-index']);
if (!docs || !issue) {
  console.error('用法: node check-docs.mjs --docs <仓库>/docs/需求 --issue <需求号> [--skip-index]');
  process.exit(2);
}

const docsDir = path.resolve(docs);
// --docs 指向 docs/需求 时，流程目录与总索引都在其父级 docs/ 下。
const docsRoot = ['需求', '40-需求'].includes(path.basename(docsDir)) ? path.dirname(docsDir) : docsDir;
const repoRoot = path.dirname(docsRoot);
const errors = [];
const infos = [];
const fail = (file, line, msg) => errors.push(`${file}${line ? `:${line}` : ''}  ${msg}`);

// 拆出链接目标的文件部分与行号锚点：支持 `/path/f.go:12`、`./f.go#L12`、`f.go`。
function splitTarget(target) {
  const withoutFragment = target.split('#')[0];
  const lineMatch = target.match(/(?:#L|:)(\d+)$/) || target.match(/#(\d+)$/);
  return { file: withoutFragment.replace(/:\d+$/, ''), line: lineMatch ? Number(lineMatch[1]) : 0 };
}

// 相对链接按文档所在目录解析；绝对路径不存在时，按仓内锚点回退到 docs 所在仓库根目录（worktree / 多根目录场景）。
function resolveTarget(target, fromDir = docsDir) {
  const { file, line } = splitTarget(target);
  const raw = path.isAbsolute(file) ? file : path.resolve(fromDir, file);
  if (fs.existsSync(raw)) return { path: raw, line };
  const anchor = file.includes('/app/') ? file.slice(file.indexOf('/app/')) : file.includes('/docs/') ? file.slice(file.indexOf('/docs/')) : null;
  if (anchor) {
    const alt = path.join(repoRoot, anchor);
    if (fs.existsSync(alt)) return { path: alt, line };
  }
  return { path: raw, line };
}

// 行号锚点必须落在文件内：文档引用代码行时，行号腐化比路径写错更常见。
function checkTargetLine(resolved, file, idx) {
  if (!resolved.line || !fs.existsSync(resolved.path)) return;
  let stat;
  try {
    stat = fs.statSync(resolved.path);
  } catch {
    return;
  }
  if (!stat.isFile() || stat.size > 4 << 20) return;
  const count = fs.readFileSync(resolved.path, 'utf8').split('\n').length;
  if (resolved.line > count) fail(file, idx + 1, `链接行号超出文件行数: ${resolved.path.split('/').pop()} 只有 ${count} 行，引用第 ${resolved.line} 行`);
}

function countUnescapedPipes(line) {
  let count = 0;
  for (let i = 0; i < line.length; i++) {
    if (line[i] === '|' && line[i - 1] !== '\\') count++;
  }
  return count;
}

function checkTables(file, lines) {
  let headerPipes = 0;
  let inTable = false;
  lines.forEach((line, idx) => {
    const isRow = /^\s*\|/.test(line);
    if (!isRow) {
      inTable = false;
      return;
    }
    const pipes = countUnescapedPipes(line);
    if (!inTable) {
      inTable = true;
      headerPipes = pipes;
      return;
    }
    if (pipes !== headerPipes) {
      fail(file, idx + 1, `表格列数不一致（表头 ${headerPipes - 1} 列，本行 ${pipes - 1} 列），检查单元格内未转义的 |`);
    }
  });
}

function checkLinks(file, lines, selfName, fromDir = docsDir) {
  const linkRe = /\[([^\]]+)\]\(([^)]+)\)/g;
  lines.forEach((line, idx) => {
    let m;
    while ((m = linkRe.exec(line))) {
      const target = m[2];
      if (/^(https?:|mailto:|#)/.test(target)) continue;
      const resolved = resolveTarget(target, fromDir);
      if (!fs.existsSync(resolved.path)) fail(file, idx + 1, `链接目标不存在: ${target}`);
      else checkTargetLine(resolved, file, idx);
      const base = path.basename(splitTarget(target).file);
      if (base === selfName) fail(file, idx + 1, `文档链接了自身: ${target}`);
    }
  });
}

function checkImages(file, lines, docDir, assetsDir = null) {
  const imgRe = /!\[[^\]]*\]\(([^)]+)\)/g;
  lines.forEach((line, idx) => {
    let m;
    while ((m = imgRe.exec(line))) {
      const target = m[1].trim();
      if (/^https?:/.test(target)) continue;
      const resolved = path.resolve(docDir, target);
      if (!fs.existsSync(resolved)) {
        fail(file, idx + 1, `图片不存在: ${target}`);
        continue;
      }
      if (assetsDir) {
        const rel = path.relative(assetsDir, resolved);
        if (rel.startsWith('..') || path.isAbsolute(rel)) {
          fail(file, idx + 1, `图片应放在需求目录 assets/ 下: ${target}`);
        }
      }
    }
  });
}

function pickIndex(dir) {
  for (const candidate of INDEX_CANDIDATES) {
    const full = path.join(dir, candidate);
    if (fs.existsSync(full) && fs.statSync(full).isFile()) return { full, name: candidate };
  }
  return { full: path.join(dir, INDEX_CANDIDATES[0]), name: INDEX_CANDIDATES[0] };
}

function linkTargets(text) {
  const out = [];
  const re = /\[[^\]]*\]\(([^)]+)\)/g;
  let m;
  while ((m = re.exec(text))) {
    let target = m[1].trim().replace(/^<|>$/g, '');
    try {
      target = decodeURIComponent(target);
    } catch {
      // 保留原始目标，交给 basename 比对。
    }
    out.push(path.basename(splitTarget(target).file));
  }
  return out;
}

function checkIndexLink(display, full, expected) {
  const hits = linkTargets(fs.readFileSync(full, 'utf8')).filter((t) => t === expected).length;
  if (!hits) fail(display, null, `索引缺少「${expected}」链接`);
  else if (hits > 1) fail(display, null, `索引存在重复条目「${expected}」（应按需求号原地替换，不追加第二行）`);
}

// 索引按需求号维护：主索引列五件套；缺条目或重复条目都视为未同步。
function checkIndexes(fiveDocs) {
  if (skipIndex) return;
  const names = new Set([...fiveDocs.values()].map((v) => v.name));
  const name = names.size === 1 ? [...names][0] : null;
  if (!name) return;

  const main = pickIndex(docsRoot);
  if (!fs.existsSync(main.full)) {
    errors.push(`缺少文档总索引 docs/${main.name}（新增需求文档后同步更新）`);
  } else {
    for (const type of TYPES) checkIndexLink(`docs/${main.name}`, main.full, `${issue}-${name}-${type}.md`);
  }
}

function headingSlug(title) {
  return title
    .trim()
    .toLowerCase()
    .replace(/<[^>]*>/g, '')
    .replace(/[`*_~]/g, '')
    .replace(/[^\p{L}\p{N}\s-]/gu, '')
    .replace(/\s+/g, '-');
}

// 目录必须使用标题自动锚点；HTML <a id> 不作为合法目录锚点。
function checkToc(file, lines) {
  const start = lines.findIndex((line) => line.trim() === '## 目录');
  if (start < 0) return;

  let end = lines.length;
  for (let i = start + 1; i < lines.length; i++) {
    if (/^##\s+/.test(lines[i])) {
      end = i;
      break;
    }
  }

  const anchors = new Set();
  const counts = new Map();
  for (const line of lines) {
    const match = line.match(/^(##|###)\s+(.+)$/);
    if (!match || line.trim() === '## 目录') continue;
    const base = headingSlug(match[2]);
    const count = counts.get(base) || 0;
    counts.set(base, count + 1);
    anchors.add(count === 0 ? base : `${base}-${count}`);
  }

  const targets = [];
  for (let i = start + 1; i < end; i++) {
    const linkRe = /\]\(#([^)]+)\)/g;
    let match;
    while ((match = linkRe.exec(lines[i]))) {
      let target = match[1];
      try {
        target = decodeURIComponent(target);
      } catch {
        // 保留原始值，交给后续存在性校验
      }
      targets.push(target);
    }
  }
  if (targets.length === 0) {
    fail(file, start + 1, '目录未包含页内锚点链接');
    return;
  }
  for (const target of targets) {
    if (!anchors.has(target)) fail(file, start + 1, `目录锚点「#${target}」没有对应标题`);
  }
}

function checkHeader(file, lines, type) {
  const block = [];
  for (const line of lines) {
    if (!line.startsWith('>')) {
      if (block.length) break;
      continue;
    }
    block.push(line);
  }
  const text = block.join('\n');

  if (type === '需求清单' && !/>\s*来源[:：]\s*\S+/.test(text)) fail(file, null, '需求清单头部缺少「来源」');

  const labelOrder = [];
  const linkRe = /\[([^\]]+)\]\(([^)]+)\)/g;
  let m;
  while ((m = linkRe.exec(text))) {
    const label = m[1].trim();
    const base = path.basename(m[2].replace(/:\d+$/, ''));
    const matchedType = TYPES.find((t) => base.startsWith(`${issue}-`) && base.endsWith(`-${t}.md`));
    if (!matchedType) continue;
    const labelOk = label === matchedType || label === base;
    if (!labelOk) fail(file, null, `链接文案「${label}」应为目标文件名或类型名「${matchedType}」`);
    labelOrder.push(TYPES.indexOf(matchedType));
  }

  if (type === '前端对接文档' && labelOrder.length) {
    fail(file, null, '前端对接文档不应链接内部文档（对外契约文档不带内部材料路径）');
  }
  if (LINKED_TYPES.includes(type)) {
    if (!labelOrder.includes(TYPES.indexOf('需求清单'))) fail(file, null, '头部缺少「需求清单」链接');
    for (let i = 1; i < labelOrder.length; i++) {
      if (labelOrder[i] < labelOrder[i - 1]) fail(file, null, '头部链接顺序须为「需求清单 → 方案评估 → 实施方案 → 方案验收」');
    }
  }

  const statusMatch = text.match(/状态[:：]\s*([^｜|\n]+)/);
  if (!statusMatch) {
    fail(file, null, '头部缺少「状态」');
  } else {
    const status = statusMatch[1].trim();
    if (STATUS_FORBIDDEN.some((w) => status.includes(w))) fail(file, null, `状态用语「${status}」不可核，禁止使用「已完成」`);
  }
}

const issueDir = path.join(docsDir, issue);
if (!fs.existsSync(issueDir) || !fs.statSync(issueDir).isDirectory()) {
  errors.push(`缺少需求目录 docs/需求/${issue}/（需求文档按需求号分目录存放）`);
}

// 旧布局的平铺文件直接指出新位置，避免只报“缺少文档类型”。
const legacy = fs.existsSync(docsDir)
  ? fs.readdirSync(docsDir).filter((f) => f.startsWith(`${issue}-`) && f.endsWith('.md'))
  : [];
if (legacy.length) {
  errors.push(`需求文档应放在 docs/需求/${issue}/ 下，不要平铺在需求根目录: ${legacy.join(', ')}`);
}

const found = new Map();
const issueEntries = fs.existsSync(issueDir) ? fs.readdirSync(issueDir, { withFileTypes: true }) : [];
for (const entry of issueEntries) {
  if (!entry.isFile()) continue;
  const m = entry.name.match(new RegExp(`^${issue}-(.+)-(${TYPES.join('|')})\\.md$`));
  if (!m) continue;
  if (found.has(m[2])) errors.push(`重复文档: ${found.get(m[2]).file} 与 ${entry.name}`);
  found.set(m[2], { file: entry.name, name: m[1] });
}

for (const t of TYPES) {
  if (!found.has(t)) errors.push(`缺少文档类型「${t}」（docs/需求/${issue}/${issue}-{中文名称}-${t}.md）`);
}

// 需求目录只允许五件套与 assets/、sql/，其余内容视为偏离约定。
const docFiles = new Set([...found.values()].map((v) => v.file));
for (const entry of issueEntries) {
  if (entry.isDirectory()) {
    if (entry.name !== 'assets' && entry.name !== 'sql') {
      errors.push(`需求目录只允许 assets/ 与 sql/ 两个子目录，发现: ${entry.name}/`);
    }
  } else if (!docFiles.has(entry.name)) {
    errors.push(
      entry.name.endsWith('.sql')
        ? `SQL 文件应放在 sql/ 下: ${entry.name}`
        : `需求目录内不应出现五件套之外的文件: ${entry.name}`,
    );
  }
}

const sqlDir = path.join(issueDir, 'sql');
if (fs.existsSync(sqlDir)) {
  if (!fs.statSync(sqlDir).isDirectory()) {
    errors.push('需求目录下的 sql 应为目录');
  } else {
    for (const f of fs.readdirSync(sqlDir)) {
      if (f.endsWith('.sql') && !/^[A-Za-z0-9_][A-Za-z0-9_.]*\.sql$/.test(f)) {
        errors.push(`SQL 文件名应为「{具体表名}.sql」: sql/${f}`);
      }
    }
  }
}

const names = new Set([...found.values()].map((v) => v.name));
if (names.size > 1) errors.push(`五份文档的中文名称不一致: ${[...names].join(' / ')}`);

for (const [type, { file, name }] of found) {
  const full = path.join(issueDir, file);
  const lines = fs.readFileSync(full, 'utf8').split('\n');
  const expectedH1 = `# ${issue} ${name} — ${type}`;
  const h1 = lines.find((l) => l.trim().length > 0) || '';
  if (h1.trim() !== expectedH1) fail(file, 1, `H1 应为「${expectedH1}」，实际「${h1.trim()}」`);
  checkTables(file, lines);
  checkLinks(file, lines, file, issueDir);
  checkImages(file, lines, issueDir, path.join(issueDir, 'assets'));
  checkHeader(file, lines, type);
  checkToc(file, lines);
  infos.push(`${type}  ${issue}/${file}  ${lines.length} 行`);
}

checkIndexes(found);

if (infos.length) console.log(infos.join('\n'));
if (errors.length) {
  console.error('\n校验失败:');
  for (const e of errors) console.error(`- ${e}`);
  process.exit(1);
}
console.log(`\nOK: 需求 ${issue} 五份文档、流程图/时序图与索引校验通过`);
