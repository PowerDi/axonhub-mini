import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const code = ts
  .transpileModule(readFileSync(new URL('./models.ts', import.meta.url), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  })
  .outputText.replace(/^import .*;\n/gm, '');
let moduleIndex = 0;

async function setup(t) {
  const notices = [];
  const invalidated = [];
  const requests = [];
  const result = { created: 0, appended: 0, warnings: [], models: [] };
  globalThis.modelsImportTest = {
    useMutation: (options) => options,
    useQueryClient: () => ({ invalidateQueries: (options) => invalidated.push(options.queryKey) }),
    useTranslation: () => ({ t: (key, values) => ({ key, values }) }),
    useErrorHandler: () => ({ handleError() {} }),
    graphqlRequest: async (query, variables) => {
      requests.push({ query, variables });
      return { importUnassociatedModels: result };
    },
    toast: Object.fromEntries(['success', 'info', 'warning'].map((level) => [level, (message) => notices.push({ level, message })])),
  };
  t.after(() => delete globalThis.modelsImportTest);
  const bindings = `const { ${Object.keys(globalThis.modelsImportTest).join(', ')} } = globalThis.modelsImportTest;\n`;
  const module = await import(`data:text/javascript;base64,${Buffer.from(bindings + code).toString('base64')}#${moduleIndex++}`);
  return { mutation: module.useImportUnassociatedModels(), notices, invalidated, requests, result };
}

test('a no-op import shows an informational notice instead of claiming an update', async (t) => {
  const { mutation, notices, invalidated } = await setup(t);
  mutation.onSuccess({ created: 0, appended: 0, warnings: ['This source is already associated, skipped.'], models: [] });
  assert.deepEqual(notices, [
    { level: 'info', message: { key: 'models.unassociated.import.noChanges', values: undefined } },
    { level: 'warning', message: 'This source is already associated, skipped.' },
  ]);
  assert.deepEqual(invalidated, [['models'], ['unassociatedChannels']]);
});

for (const counts of [
  { created: 1, appended: 0 },
  { created: 0, appended: 1 },
  { created: 1, appended: 1 },
]) {
  test(`an import with ${counts.created} created and ${counts.appended} updated reports actual changes`, async (t) => {
    const { mutation, notices, invalidated } = await setup(t);
    mutation.onSuccess({ ...counts, warnings: ['Duplicate source skipped.'], models: [] });
    assert.deepEqual(notices, [
      { level: 'success', message: { key: 'models.unassociated.import.success', values: counts } },
      { level: 'warning', message: 'Duplicate source skipped.' },
    ]);
    assert.deepEqual(invalidated, [['models'], ['unassociatedChannels']]);
  });
}

test('the import mutation preserves the backend result and its actual change counts', async (t) => {
  const { mutation, requests, result } = await setup(t);
  const items = [{ targetModelId: 1, sources: [{ channelId: 2, upstreamModelId: 'existing-source' }] }];
  assert.equal(await mutation.mutationFn(items), result);
  assert.equal(requests.length, 1);
  assert.match(requests[0].query, /importUnassociatedModels/);
  assert.deepEqual(requests[0].variables, { items });
});

test('both locales provide the no-change import notice', () => {
  for (const locale of ['en', 'zh-CN']) {
    const translations = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/models.json`, import.meta.url), 'utf8'));
    assert.equal(typeof translations['models.unassociated.import.noChanges'], 'string');
    assert.ok(translations['models.unassociated.import.noChanges'].length > 0);
  }
});
