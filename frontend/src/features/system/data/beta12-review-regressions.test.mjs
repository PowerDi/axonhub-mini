import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import ts from 'typescript';

const read = (path) => readFileSync(new URL(`../../../${path}`, import.meta.url), 'utf8');
const quotas = read('features/system/data/quotas.ts');
const badges = read('components/quota-badges.tsx');
const parsedSource = quotas.replace(/^import[^\n]*\n/gm, '').replace('function parseChannelNode(', 'export function parseChannelNode(');
const compiled = ts.transpileModule(parsedSource, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 } }).outputText;
const { parseChannelNode } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const fixture = (type, quotaData = {}) => ({ id: '1', name: 'quota fixture', type, orderingWeight: 1, settings: null,
  providerQuotaStatus: { status: 'available', ready: true, nextResetAt: null, quotaData, providerType: type },
});

test('xAI subscription parsing preserves plan, billing windows and battery percentage', () => {
  const quotaData = { plan_type: 'SuperGrok', billing: { weekly: { usage_percent: 35 }, monthly: { usage_percent: 65 } } };
  const parsed = parseChannelNode(fixture('xai_subscription', quotaData));
  assert.equal(parsed.type, 'xai_subscription');
  assert.deepEqual(parsed.quotaStatus.quotaData, quotaData);
  // Execute the production percentage function, not a copy of its calculation.
  const percentageSource = badges.match(/function getChannelPercentage\([\s\S]*?\n\}/)[0];
  const code = ts.transpileModule(percentageSource, { compilerOptions: { target: ts.ScriptTarget.ES2023 } }).outputText;
  const percentage = new Function(`${code}; return getChannelPercentage;`)();
  assert.equal(percentage(parsed), 65);
});

test('every provider rendered by quota badges keeps its discriminant', () => {
  const types = new Set([...badges.matchAll(/channel\.type === '([^']+)'/g)].map((match) => match[1]));
  const ast = ts.createSourceFile('quotas.ts', quotas, ts.ScriptTarget.Latest, true);
  const quotaType = ast.statements.find((node) => ts.isTypeAliasDeclaration(node) && node.name.text === 'ProviderQuotaChannel');
  const visit = (node) => {
    if (ts.isPropertySignature(node) && node.name.getText(ast) === 'type') {
      const members = ts.isUnionTypeNode(node.type) ? node.type.types : [node.type];
      for (const member of members) if (ts.isLiteralTypeNode(member) && ts.isStringLiteral(member.literal)) types.add(member.literal.text);
    }
    ts.forEachChild(node, visit);
  };
  visit(quotaType);
  for (const type of types) {
    if (type === 'unknown') continue;
    if (type === 'openai' || type === 'openai_responses') {
      for (const providerType of ['synthetic', 'neuralwatt', 'apertis']) {
        const node = fixture(type);
        node.providerQuotaStatus.providerType = providerType;
        assert.equal(parseChannelNode(node).type, type, `${type}/${providerType} must remain discriminated`);
      }
    } else {
      assert.equal(parseChannelNode(fixture(type)).type, type, `${type} must not fall through to unknown`);
    }
  }
  assert.equal(parseChannelNode(fixture('future-provider')).type, 'unknown');
});

test('create and duplicate submissions use the validated form without a second derived-type parse', () => {
  const dialog = read('features/channels/components/channels-action-dialog.tsx');
  const submit = dialog.slice(dialog.indexOf('const onSubmit = async'), dialog.indexOf('const addModel ='));
  assert.doesNotMatch(submit, /createChannelInputSchema\.parse\(/);
  assert.match(submit, /const createInput = \{[\s\S]*?\} as CreateChannelInput/);
  assert.match(submit, /duplicateChannel\.mutateAsync\(\{[\s\S]*?input: createInput/);
  assert.match(submit, /createChannel\.mutateAsync\(createInput\)/);
});

test('analytics header order matches all eleven colgroup and data columns', () => {
  const source = read('features/analytics/components/dimension-detail-table.tsx');
  const headers = [...source.slice(source.indexOf('<thead'), source.indexOf('</thead>')).matchAll(/analytics\.table\.(\w+)/g)].map((match) => match[1]);
  assert.deepEqual(headers, ['name', 'totalTokens', 'inputTokens', 'cachedTokens', 'uncachedTokens', 'outputTokens', 'cacheHitRate', 'requests', 'tokensPerSecond', 'ttft', 'cost']);
  assert.equal((source.match(/<col className=/g) ?? []).length, headers.length);
  assert.equal((source.match(/<td className=/g) ?? []).length, headers.length);
  assert.match(source, /item\.requestCount[\s\S]*item\.tokensPerSecond[\s\S]*item\.ttftMs[\s\S]*item\.cost/);
});

test('policy dialog reports success only after the settings mutation resolves', () => {
  const source = read('features/channels/components/channels-model-api-format-policy-dialog.tsx');
  assert.match(source, /await updateChannel\.mutateAsync\([\s\S]*?toast\.success\(t\('channels\.messages\.updateSuccess'\)\);\s*onOpenChange\(false\)/);
});

test('warning banner retains theme variables and mobile wrapping; battery never shrinks', () => {
  const banner = read('features/channels/components/channels-error-banner.tsx');
  assert.doesNotMatch(banner, /(?:border|text|bg)-orange-/);
  assert.match(banner, /border-warning\/40 bg-warning\/10 text-\(--warning-soft-fg\)/);
  assert.match(banner, /flex-col[\s\S]*sm:flex-row/);
  assert.match(banner, /self-start sm:self-auto/);
  assert.match(badges, /<BatteryIcon\s+className=\{`h-4 w-4 shrink-0/);
});

test('Playwright suites isolate discovery, dependency cache and interactive login state', () => {
  const rootRead = (path) => readFileSync(new URL(`../../../../${path}`, import.meta.url), 'utf8');
  assert.match(rootRead('playwright.config.ts'), /testIgnore: '\*\*\/ui-regression\/\*\*'/);
  assert.match(rootRead('playwright.ui.config.ts'), /--config vite\.ui\.config\.ts/);
  assert.match(rootRead('vite.ui.config.ts'), /cacheDir: 'node_modules\/\.vite-ui-regression'/);
  for (const file of ['system-claude-code.spec.ts', 'system-codex-compatibility.spec.ts']) {
    assert.match(rootRead(`tests/${file}`), /test\.use\(\{ storageState: \{ cookies: \[\], origins: \[\] \} \}\)/);
  }
});
