import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { QueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import ts from 'typescript';
import { pageInfoSchema } from '../../../gql/pagination.ts';

const code = ts
  .transpileModule(readFileSync(new URL('./channels.ts', import.meta.url), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  })
  .outputText.replace(/^import .*;\n/gm, '');
let moduleIndex = 0;

const cases = [
  ...['disabled', 'archived', 'enabled'].map((status) => ({
    name: `set channel ${status}`,
    hook: 'useUpdateChannelStatus',
    operation: 'updateChannelStatus',
    input: { id: 'channel-1', status },
    variables: { id: 'channel-1', status },
  })),
  ...['Archive', 'Disable', 'Enable', 'Recover', 'Delete'].map((action) => ({
    name: `bulk ${action.toLowerCase()}`,
    hook: `useBulk${action}Channels`,
    operation: `bulk${action}Channels`,
    input: ['channel-1', 'channel-2'],
    variables: { ids: ['channel-1', 'channel-2'] },
    deleted: action === 'Delete',
  })),
  { name: 'delete channel', hook: 'useDeleteChannel', operation: 'deleteChannel', input: 'channel-1', variables: { id: 'channel-1' }, deleted: true },
];

const relatedKeys = [
  ['channels', { page: 1 }],
  ['allChannelTags'],
  ['allChannelSummarys', undefined, undefined],
  ['allChannelSummarys', undefined, true],
  ['allChannelSummarys', 'project-1', true],
  ['models', { page: 1 }],
  ['models', { page: 2 }],
  ['unassociatedChannels'],
];
const settingsKey = ['modelSettings'];
const unrelatedKey = ['unrelatedData'];

async function setup(t, scenario, failure) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity }, mutations: { retry: false } } });
  for (const key of [...relatedKeys, settingsKey, unrelatedKey]) {
    queryClient.setQueryData(key, { saved: true });
  }
  const requests = [];
  const errors = [];
  const notices = [];
  globalThis.channelLifecycleTest = {
    z,
    pageInfoSchema,
    useMutation: (options) => options,
    useQueryClient: () => queryClient,
    useTranslation: () => ({ t: (key) => key }),
    useErrorHandler: () => ({ handleError: (error) => errors.push(error) }),
    toast: { success: (message) => notices.push(message) },
    graphqlRequest: async (query, variables) => {
      requests.push({ query, variables });
      if (failure) throw failure;
      return { [scenario.operation]: true };
    },
  };
  t.after(() => {
    queryClient.clear();
    delete globalThis.channelLifecycleTest;
  });
  const bindings = `const { ${Object.keys(globalThis.channelLifecycleTest).join(', ')} } = globalThis.channelLifecycleTest;\n`;
  const module = await import(`data:text/javascript;base64,${Buffer.from(bindings + code).toString('base64')}#${moduleIndex++}`);
  const mutation = queryClient.getMutationCache().build(queryClient, module[scenario.hook]());
  return { queryClient, requests, errors, notices, execute: () => mutation.execute(scenario.input) };
}

for (const scenario of cases) {
  test(`${scenario.name} refreshes association data after success`, async (t) => {
    const { queryClient, requests, errors, notices, execute } = await setup(t, scenario);
    assert.equal(await execute(), true);
    assert.equal(requests.length, 1);
    assert.ok(requests[0].query.includes(`${scenario.operation}(`));
    assert.deepEqual(requests[0].variables, scenario.variables);
    for (const key of relatedKeys) {
      assert.equal(queryClient.getQueryState(key).isInvalidated, true, JSON.stringify(key));
    }
    assert.equal(queryClient.getQueryState(settingsKey).isInvalidated, !!scenario.deleted);
    assert.equal(queryClient.getQueryState(unrelatedKey).isInvalidated, false);
    assert.deepEqual(errors, []);
    assert.equal(notices.length, 1);
  });

  test(`${scenario.name} preserves cached data and reports a failed request`, async (t) => {
    const failure = new Error('request failed');
    const { queryClient, errors, notices, execute } = await setup(t, scenario, failure);
    await assert.rejects(execute, (error) => error === failure);
    for (const key of [...relatedKeys, settingsKey, unrelatedKey]) {
      assert.equal(queryClient.getQueryState(key).isInvalidated, false, JSON.stringify(key));
      assert.deepEqual(queryClient.getQueryData(key), { saved: true });
    }
    assert.deepEqual(errors, [failure]);
    assert.deepEqual(notices, []);
  });
}
