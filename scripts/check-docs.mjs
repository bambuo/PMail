#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";

const args = parseArgs(process.argv.slice(2));
if (args.help) {
  printHelp();
  process.exit(0);
}

const root = path.resolve(args.root || process.cwd());
const docsDir = path.resolve(root, args.docs || "docs");
const flowDirName = args.flowDir === "" ? null : (args.flowDir || "流程");
const requireFlowVersion = !args.noFlowVersion;
const requireFlowDiagrams = Boolean(args.requireFlowDiagrams);
const requireStatus = Boolean(args.requireStatus);
const requireEvidence = Boolean(args.requireEvidence);

if (!fs.existsSync(docsDir) || !fs.statSync(docsDir).isDirectory()) {
  console.error(`Docs directory not found: ${docsDir}`);
  process.exit(2);
}

const failures = [];
const files = walk(docsDir).filter((f) => f.toLowerCase().endsWith(".md")).sort();

for (const file of files) {
  const rel = path.relative(root, file);
  const text = fs.readFileSync(file, "utf8");
  const base = path.basename(file);
  const isReadme = /^README\.md$/i.test(base);
  const isFixedDoc = isReadme || base === "架构全景图.md";
  const inFlowDir = flowDirName
    ? path.relative(docsDir, file).split(path.sep).includes(flowDirName)
    : false;
  // 需求文档按需求号目录存放，命名与五件套结构由 requirement-docs 的脚本校验。
  const topDir = path.relative(docsDir, file).split(path.sep)[0];
  const inRequirementDir = topDir === "40-需求";
  const inArchiveDir = topDir === "90-归档";

  if (!isFixedDoc && !inRequirementDir && !inArchiveDir) checkNaming(file, rel, base, text, inFlowDir);
  if (!isFixedDoc && inFlowDir && requireFlowDiagrams) checkDiagrams(rel, text);
  checkLinks(file, rel, text);
  checkTables(rel, text);
}

checkRequirementLayout();

if (failures.length) {
  for (const f of failures) console.log(`FAIL ${f}`);
  console.log(`\nDocs check: ${files.length} files, ${failures.length} failure(s)`);
  process.exit(1);
}

console.log(`Docs check: ${files.length} files, 0 failures`);

function checkRequirementLayout() {
  const requirementDir = path.join(docsDir, "40-需求");
  if (!fs.existsSync(requirementDir)) return;
  if (!fs.statSync(requirementDir).isDirectory()) {
    fail(path.relative(root, requirementDir), "需求应为目录");
    return;
  }
  for (const entry of fs.readdirSync(requirementDir, { withFileTypes: true })) {
    const issuePath = path.join(requirementDir, entry.name);
    if (!entry.isDirectory()) {
      fail(path.relative(root, issuePath), "需求文档应按需求号分目录存放，不要平铺在需求根目录");
      continue;
    }
    if (!/^\d{4}$/.test(entry.name)) {
      fail(path.relative(root, issuePath), "需求目录名应为四位需求号");
      continue;
    }
    for (const child of fs.readdirSync(issuePath, { withFileTypes: true })) {
      const childPath = path.join(issuePath, child.name);
      if (child.isDirectory()) {
        if (child.name !== "assets" && child.name !== "sql") {
          fail(path.relative(root, childPath), "需求号目录只允许 assets/ 与 sql/ 两个子目录");
        }
        continue;
      }
      if (/\.sql$/i.test(child.name)) {
        fail(path.relative(root, childPath), "SQL 文件应放在 sql/ 子目录");
      } else if (!/\.md$/i.test(child.name)) {
        fail(path.relative(root, childPath), "图片等资源应放在 assets/ 子目录");
      }
    }
  }
}

function checkNaming(file, rel, base, text, inFlowDir) {
  const mainMatch = base.match(/^(\d{2})-(.+)\.md$/);
  if (!mainMatch) {
    fail(rel, "文件名缺少两位编号或 .md 后缀");
    return;
  }

  const number = mainMatch[1];
  let name = mainMatch[2];
  let version = null;
  const versionMatch = name.match(/^(.+)-v(\d+)$/);
  if (versionMatch) {
    name = versionMatch[1];
    version = versionMatch[2];
  }

  if (inFlowDir) {
    if (requireFlowVersion && !version) {
      fail(rel, "流程图文件名缺少 -vN 版本后缀");
    }
  } else if (version) {
    fail(rel, "主文档文件名不应带 -vN 版本后缀");
  }

  const h1 = text.split(/\r?\n/, 1)[0].trim();
  const h1Match = h1.match(/^#\s+(\d{2})\s+(.+)$/);
  if (!h1Match) {
    fail(rel, "一级标题缺少两位编号");
    return;
  }

  const h1Number = h1Match[1];
  let h1Name = h1Match[2].trim();
  let h1Version = null;
  const h1VersionMatch = h1Name.match(/^(.+?)(?:（v(\d+)）|\(v(\d+)\)|\s+v(\d+))$/);
  if (h1VersionMatch) {
    h1Name = h1VersionMatch[1].trim();
    h1Version = h1VersionMatch[2] || h1VersionMatch[3] || h1VersionMatch[4];
  }

  if (h1Number !== number) fail(rel, `H1 编号 ${h1Number} 与文件名编号 ${number} 不一致`);
  if (h1Name !== name) fail(rel, `H1 名称“${h1Name}”与文件名名称“${name}”不一致`);
  if (version || h1Version) {
    if (version !== h1Version) fail(rel, `文件名版本 v${version || "?"} 与 H1 版本 v${h1Version || "?"} 不一致`);
  }

  if (!text.includes(`> 文档编号：${number}`)) fail(rel, `缺少文档头“> 文档编号：${number}”`);
  if (version && !text.includes(`> 文档版本：v${version}`)) fail(rel, `缺少文档头“> 文档版本：v${version}”`);
  if (requireStatus && !text.includes("> 状态：")) fail(rel, "缺少文档头“> 状态：”");
  if (requireEvidence && !text.includes("> 证据基线：")) fail(rel, "缺少文档头“> 证据基线：”");
}

function checkDiagrams(rel, text) {
  const blocks = [...text.matchAll(/```mermaid\s*\n([\s\S]*?)```/g)].map((m) => m[1]);
  if (!blocks.length) {
    fail(rel, "未找到 Mermaid 图");
    return;
  }
  for (const block of blocks) {
    if (!block.trim()) fail(rel, "存在空的 Mermaid 图");
  }
  const joined = blocks.join("\n");
  if (!/```mermaid\s*\nflowchart\b/.test(text)) fail(rel, "缺少 flowchart 流程图");
  if (!/```mermaid\s*\nsequenceDiagram\b/.test(text)) fail(rel, "缺少 sequenceDiagram 时序图");
  if (!/```mermaid\s*\nstateDiagram-v2\b/.test(text)) fail(rel, "缺少 stateDiagram-v2 状态机");
  void joined;
}

function checkLinks(file, rel, text) {
  const clean = stripFences(text);
  const re = /\[[^\]]*\]\(([^)]+)\)/g;
  let match;
  while ((match = re.exec(clean))) {
    let target = match[1].trim();
    if (!target || /^(https?:|mailto:|tel:|data:|#)/i.test(target)) continue;
    target = target.replace(/^<|>$/g, "");
    target = target.split("#")[0].split("?")[0];
    if (!target) continue;
    try {
      target = decodeURIComponent(target);
    } catch {
      // Keep the raw target when it is not valid percent-encoding.
    }
    const resolved = target.startsWith("/")
      ? path.resolve(root, "." + target)
      : path.resolve(path.dirname(file), target);
    if (!fs.existsSync(resolved)) fail(rel, `链接目标不存在：${match[1]}`);
  }
}

function checkTables(rel, text) {
  const lines = stripFences(text).split(/\r?\n/);
  let block = [];
  let start = 0;
  const flush = () => {
    if (block.length < 2) {
      block = [];
      return;
    }
    const counts = block.map((line) => (line.replace(/\\\|/g, "").match(/\|/g) || []).length);
    const expected = counts[0];
    counts.forEach((count, i) => {
      if (count !== expected) fail(rel, `表格第 ${start + i + 1} 行列数不一致`);
    });
    if (!/^\|[\s:|-]+\|$/.test(block[1].trim())) {
      fail(rel, `表格第 ${start + 2} 行缺少分隔行`);
    }
    block = [];
  };
  lines.forEach((line, i) => {
    if (line.startsWith("|")) {
      if (!block.length) start = i;
      block.push(line);
    } else {
      flush();
    }
  });
  flush();
}

function stripFences(text) {
  return text.replace(/```[\s\S]*?```/g, (m) => m.replace(/[^\n]/g, ""));
}

function walk(dir) {
  const out = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === ".git" || entry.name === "node_modules") continue;
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) out.push(...walk(p));
    else if (entry.isFile()) out.push(p);
  }
  return out;
}

function fail(file, message) {
  failures.push(`${file}: ${message}`);
}

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--help" || a === "-h") out.help = true;
    else if (a === "--docs") out.docs = argv[++i];
    else if (a.startsWith("--docs=")) out.docs = a.slice("--docs=".length);
    else if (a === "--root") out.root = argv[++i];
    else if (a.startsWith("--root=")) out.root = a.slice("--root=".length);
    else if (a === "--flow-dir") out.flowDir = argv[++i];
    else if (a.startsWith("--flow-dir=")) out.flowDir = a.slice("--flow-dir=".length);
    else if (a === "--require-flow-diagrams") out.requireFlowDiagrams = true;
    else if (a === "--require-status") out.requireStatus = true;
    else if (a === "--require-evidence") out.requireEvidence = true;
    else if (a === "--no-flow-version") out.noFlowVersion = true;
    else {
      console.error(`Unknown argument: ${a}`);
      printHelp();
      process.exit(2);
    }
  }
  return out;
}

function printHelp() {
  console.log(`Usage: node check-docs.mjs [options]

Options:
  --docs <dir>               Docs directory (default: docs)
  --root <dir>               Repository root (default: cwd)
  --flow-dir <name>          Flow subdirectory name (default: 流程)
  --require-flow-diagrams    Require flowchart + sequenceDiagram + stateDiagram-v2
  --require-status           Require "> 状态：" in document headers
  --require-evidence         Require "> 证据基线：" in document headers
  --no-flow-version          Do not require -vN suffix for flow documents
  -h, --help                 Show this help`);
}
