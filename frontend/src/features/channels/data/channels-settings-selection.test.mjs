// Structural guard: every `settings { ... }` selection block in channels.ts must
// request `quotaRoutingMode`, so a new or edited query/mutation cannot silently
// drop the field and starve the zod schema / merge whitelist downstream.
import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const source = readFileSync(join(import.meta.dirname, 'channels.ts'), 'utf8');
const lines = source.split('\n');

function enclosingDeclaration(lineIndex) {
  for (let i = lineIndex; i >= 0; i--) {
    const m = lines[i].match(/const (\w+)\s*=/);
    if (m) return m[1];
  }
  return `line ${lineIndex + 1}`;
}

function extractSettingsBlocks() {
  const blocks = [];
  for (let i = 0; i < lines.length; i++) {
    const open = lines[i].indexOf('settings {');
    if (open === -1) continue;
    let depth = 0;
    let body = '';
    for (let j = i; j < lines.length; j++) {
      const line = j === i ? lines[j].slice(open) : lines[j];
      for (const ch of line) {
        if (ch === '{') depth++;
        else if (ch === '}') depth--;
      }
      body += `${line}\n`;
      if (depth === 0) break;
    }
    blocks.push({ name: enclosingDeclaration(i), body });
  }
  return blocks;
}

test('every settings selection block in channels.ts requests quotaRoutingMode', () => {
  const blocks = extractSettingsBlocks();
  assert.ok(blocks.length >= 1, 'no `settings {` blocks found — selection extraction is broken');

  const missing = blocks.filter((b) => !/\bquotaRoutingMode\b/.test(b.body));
  assert.deepEqual(
    missing.map((b) => b.name),
    [],
    `settings selection block(s) missing quotaRoutingMode: ${missing.map((b) => b.name).join(', ')}`
  );
});

test('full settings selections return both mini policies and beta12 retry/key settings', () => {
  // Column-only projections intentionally omit editor settings; edit dialogs fetch full detail.
  const fullBlocks = extractSettingsBlocks().filter(({ name }) => !['CHANNEL_QUERY_ROUTING_STATUS_SELECTION', 'CHANNEL_QUERY_PROXY_SELECTION'].includes(name));
  assert.ok(fullBlocks.length > 0);
  for (const field of ['modelApiFormatPolicies', 'allow', 'exclude', 'hammerRetry', 'reasoningFilter', 'maxSingleChannelRetries', 'responseTimeout', 'apiKeyStrategy', 'apiKeyRoundRobinSwitchAfter']) {
    const missing = fullBlocks.filter(({ body }) => !new RegExp(`\\b${field}\\b`).test(body));
    assert.deepEqual(missing.map(({ name }) => name), [], `settings selection missing ${field}`);
  }
});

test('all full channel settings selections return independent entry fields', () => {
  const fullBlocks = extractSettingsBlocks().filter(({ body }) => /\bextraModelPrefix\b/.test(body));
  assert.ok(fullBlocks.length > 0);
  for (const field of ['independentListing', 'independentDisplayName']) {
    const missing = fullBlocks.filter(({ body }) => !new RegExp(`\\b${field}\\b`).test(body));
    assert.deepEqual(missing.map(({ name }) => name), [], `full settings selection missing ${field}`);
  }
});

test('independent entries survive unrelated edits and explicit disable clears the switch', async () => {
  const { default: ts } = await import('typescript');
  const mergeSource = readFileSync(join(import.meta.dirname, '../utils/merge.ts'), 'utf8');
  const { outputText } = ts.transpileModule(mergeSource, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  });
  const { mergeChannelSettingsForUpdate } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
  const existing = { extraModelPrefix: 'claude官池', independentListing: true, independentDisplayName: 'claude官池' };
  const saved = mergeChannelSettingsForUpdate(existing, { hideOriginalModels: true });
  assert.equal(saved.independentListing, true);
  assert.equal(saved.independentDisplayName, 'claude官池');
  assert.equal(saved.extraModelPrefix, 'claude官池');
  const disabled = mergeChannelSettingsForUpdate(saved, { independentListing: false, independentDisplayName: '' });
  assert.equal(disabled.independentListing, false);
  assert.equal(disabled.independentDisplayName, '');
  assert.equal(disabled.extraModelPrefix, 'claude官池');
});

test('independent dialog preview and save use separate label and calling IDs', () => {
  const dialog = readFileSync(join(import.meta.dirname, '../components/channels-model-mapping-dialog.tsx'), 'utf8');
  assert.match(dialog, /displayName:\s*`\$\{entry\.actualModel\} · \$\{previewSettings\.independentDisplayName\?\.trim\(\) \|\| currentRow\.name\}`/);
  assert.match(dialog, /independentListing: values\.independentListing \?\? false/);
  assert.match(dialog, /independentDisplayName: values\.independentDisplayName\?\.trim\(\) \?\? ''/);
  assert.match(dialog, /data-testid='independent-model-preview'/);
  assert.match(dialog, /entry\.requestModel/);
});

test('preview label follows Magpie style without changing calling or upstream IDs', () => {
  const dialog = readFileSync(join(import.meta.dirname, '../components/channels-model-mapping-dialog.tsx'), 'utf8');
  const expression = dialog.match(/displayName:\s*(`\$\{entry\.actualModel\}[^`]+`)/)?.[1];
  assert.ok(expression, 'independent preview label expression is missing');
  const renderLabel = new Function('entry', 'previewSettings', 'currentRow', `return ${expression};`);
  const entry = { requestModel: 'claude官池/claude-opus-5-5', actualModel: 'claude-opus-5-5', source: 'prefix' };
  const before = { ...entry };
  assert.equal(renderLabel(entry, { independentDisplayName: 'claude官池' }, { name: 'Internal Pool' }), 'claude-opus-5-5 · claude官池');
  assert.equal(renderLabel(entry, { independentDisplayName: '  ' }, { name: 'claude官池' }), 'claude-opus-5-5 · claude官池');
  assert.equal(renderLabel(entry, { independentDisplayName: 'New label' }, { name: 'Internal Pool' }), 'claude-opus-5-5 · New label');
  assert.deepEqual(entry, before, 'editing the label must not mutate the calling or upstream ID');
});

test('independent entry labels have matching English and Chinese locale keys', () => {
  const english = JSON.parse(readFileSync(join(import.meta.dirname, '../../../locales/en/channels.json'), 'utf8'));
  const chinese = JSON.parse(readFileSync(join(import.meta.dirname, '../../../locales/zh-CN/channels.json'), 'utf8'));
  const prefix = 'channels.dialogs.settings.independentModels.';
  const keys = Object.keys(english).filter((key) => key.startsWith(prefix));
  assert.ok(keys.length > 0);
  assert.deepEqual(keys.sort(), Object.keys(chinese).filter((key) => key.startsWith(prefix)).sort());
  for (const key of keys) {
    assert.equal(typeof english[key], 'string');
    assert.equal(typeof chinese[key], 'string');
  }
});
