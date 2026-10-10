import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import {
  getApiFormatsForProvider,
  getAvailableProtocolFormats,
  getChannelTypeForApiFormat,
  getConfigurableApiFormatsForChannelType,
} from './protocol-options.ts';
import { getApiPath } from '../../requests/utils/curl-paths.ts';

const dataDir = import.meta.dirname;
const frontendRoot = join(dataDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(frontendRoot, relativePath), 'utf8');
}

function parseLocale(locale, file) {
  return JSON.parse(read(`locales/${locale}/${file}`));
}

const providerConfigs = {
  zenmux: { channelTypes: ['zenmux', 'zenmux_responses', 'zenmux_video'] },
  openai: { channelTypes: ['openai', 'openai_responses'] },
};

const channelConfigs = {
  zenmux: { apiFormat: 'openai/chat_completions' },
  zenmux_responses: { apiFormat: 'openai/responses' },
  zenmux_anthropic: { apiFormat: 'anthropic/messages' },
  zenmux_gemini: { apiFormat: 'gemini/contents' },
  zenmux_video: { apiFormat: 'zenmux/video' },
  openai: { apiFormat: 'openai/chat_completions' },
  openai_responses: { apiFormat: 'openai/responses' },
};

const configs = { providerConfigs, channelConfigs };

test('includes ZenMux native video in the add-channel provider formats', () => {
  assert.deepEqual(getApiFormatsForProvider('zenmux', configs), ['openai/chat_completions', 'openai/responses', 'zenmux/video']);
});

test('maps ZenMux native video back to the ZenMux channel type', () => {
  assert.equal(getChannelTypeForApiFormat('zenmux', 'zenmux/video', configs), 'zenmux');
});

test('exposes the native video default endpoint to the endpoints dialog', () => {
  assert.deepEqual(getAvailableProtocolFormats([{ apiFormat: 'zenmux/video' }], []), ['zenmux/video']);
});

test('does not expose ZenMux native video to unrelated providers', () => {
  assert.deepEqual(getApiFormatsForProvider('openai', configs), ['openai/chat_completions', 'openai/responses']);
  assert.equal(getChannelTypeForApiFormat('openai', 'zenmux/video', configs), undefined);
});

test('does not expose ZenMux native video to a provider lacking the ZenMux channel type', () => {
  const openaiOnly = { providerConfigs: { zenmux: { channelTypes: ['zenmux_responses', 'zenmux_anthropic', 'zenmux_gemini'] } }, channelConfigs };
  assert.deepEqual(getApiFormatsForProvider('zenmux', openaiOnly), ['openai/responses', 'anthropic/messages', 'gemini/contents']);
  assert.equal(getChannelTypeForApiFormat('zenmux', 'zenmux/video', openaiOnly), undefined);
});

test('preserves the existing reverse mapping for known formats', () => {
  assert.equal(getChannelTypeForApiFormat('zenmux', 'openai/responses', configs), 'zenmux_responses');
});

const configurableFormats = ['openai/chat_completions', 'openai/responses', 'typesafe/systemone', 'openai/decisions'];

test('shows the OpenAI Decisions endpoint only for the OpenAI create/edit channel type', () => {
  const openai = getConfigurableApiFormatsForChannelType('openai', configurableFormats);
  assert.ok(openai.includes('openai/decisions'), 'openai channel type should offer openai/decisions');

  for (const channelType of ['openai_responses', 'anthropic', 'gemini', 'zenmux', 'typesafe', 'codex', 'xai']) {
    const formats = getConfigurableApiFormatsForChannelType(channelType, configurableFormats);
    assert.ok(!formats.includes('openai/decisions'), `${channelType} must not offer openai/decisions in the UI`);
  }
});

test('keeps the SystemOne endpoint option unchanged when openai/decisions is offered', () => {
  assert.deepEqual(getConfigurableApiFormatsForChannelType('typesafe', configurableFormats), ['typesafe/systemone']);
  assert.ok(!getConfigurableApiFormatsForChannelType('openai', configurableFormats).includes('typesafe/systemone'));
});

test('accepts openai/decisions in the API-format schema and configurable endpoint list', () => {
  const schema = read('features/channels/data/schema.ts');
  assert.match(schema, /apiFormatSchema[\s\S]*'openai\/decisions'/, 'apiFormatSchema should accept openai/decisions');
  assert.match(
    schema,
    /configurableChannelEndpointApiFormats[^;]*'openai\/decisions'/,
    'the endpoints dialog list should offer openai/decisions'
  );
});

test('routes the Decisions request to /v1/decisions for generated cURL', () => {
  assert.equal(getApiPath('openai/decisions'), '/v1/decisions');
});

test('localizes the openai/decisions endpoint label in channels and models locales', () => {
  for (const locale of ['en', 'zh-CN']) {
    const channels = parseLocale(locale, 'channels.json');
    const models = parseLocale(locale, 'models.json');
    assert.ok(channels['channels.dialogs.fields.apiFormat.formats.openai/decisions'], `${locale} channels label missing`);
    assert.ok(models['models.dialogs.association.conditions.formatOptions.openai/decisions'], `${locale} models label missing`);
  }
});

test('keeps the openai/decisions association format listed for model association conditions', () => {
  const dialog = read('features/models/components/models-association-dialog.tsx');
  assert.match(dialog, /requestFormatConditionOptions[\s\S]*'openai\/decisions'/, 'association format options should include openai/decisions');
});
