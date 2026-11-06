import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// The endpoints dialog is the second Codex entry point: it must project the
// Responses family through the same WebSocket checkbox and serialization as the
// channel form, while every other channel keeps the generic endpoint editor.
const source = readFileSync(new URL('./channels-endpoints-dialog.tsx', import.meta.url), 'utf8');
const ast = ts.createSourceFile('channels-endpoints-dialog.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

const H = 'openai/responses';
const W = 'openai/responses-ws';

function between(start, end) {
  const startIndex = source.indexOf(start);
  assert.notEqual(startIndex, -1, `missing ${start}`);
  const endIndex = source.indexOf(end, startIndex);
  assert.notEqual(endIndex, -1, `missing ${end}`);
  return source.slice(startIndex, endIndex);
}

test('legacy restrictions use the mini policy translator without a second policy editor', () => {
  assert.match(source, /translateLegacyCodexModelPolicies\(channel, channel\.settings\?\.modelApiFormatPolicies\)/);
  assert.doesNotMatch(source, /modelProtocols|ModelProtocol|setNewProtocol/);
});

test('Codex shows a fixed Responses label with the shared WebSocket checkbox, derived URL and upgrade note', () => {
  const block = between('{isCodexChannel && (', '{/* Default endpoints */}');
  assert.match(block, /t\('channels\.dialogs\.fields\.apiFormat\.formats\.openai\/responses'\)/);
  assert.match(block, /justify-between/);
  assert.match(block, /<label[^>]*>\s*<Checkbox/, 'clicking the label toggles the checkbox');
  assert.match(block, /checked=\{codexResponsesWebSocket\}/);
  assert.match(block, /onCheckedChange=\{\(checked\) => handleCodexWebSocketChange\(checked === true\)\}/);
  assert.match(block, /data-testid='codex-endpoints-websocket-checkbox'/);
  assert.match(block, /t\('channels\.dialogs\.fields\.apiFormat\.codexWebSocket\.label'\)/);
  assert.match(block, /data-testid='codex-endpoints-websocket-url'/);
  assert.match(block, /data-testid='codex-endpoints-websocket-upgrade-note'/);
  assert.doesNotMatch(block, /<Input/, 'Codex must not get an editable WebSocket address');
});

test('Codex projection starts from the persisted endpoints and derives the WebSocket URL from the HTTP endpoint', () => {
  assert.match(source, /getCodexResponsesWebSocketState\(channel\)/);
  assert.match(source, /getCodexResponsesWebSocketURL\(toHttpBaseURL\(channel\.baseURL\), endpoints\)/);
});

test('Codex built-in Responses defaults are projected, not listed or validated as plain defaults', () => {
  assert.match(source, /isCodexChannel \? defaults\.filter\(\(ep\) => !isResponsesFamilyFormat\(ep\.apiFormat\)\) : defaults/);
});

test('both Codex entry points share one serialization routine and the save sends the projected endpoints', () => {
  assert.match(source, /isCodexChannel \? buildCodexResponsesEndpoints\(endpoints, codexResponsesWebSocket\) : endpoints/);
  const save = between('const handleSave = useCallback', 'const handleKeyDown = useCallback');
  assert.match(save, /const apiFormats = savedEndpoints\.map\(\(ep\) => ep\.apiFormat\)/);
  assert.match(save, /endpoints: savedEndpoints\.map\(\(ep\) => \(\{/);
  assert.match(save, /transport: ep\.transport \|\| undefined/, 'non-Codex legacy transport is still sent unchanged');
  assert.match(save, /isCodexChannel \? toHttpBaseURL\(channel\.baseURL\) : channel\.baseURL/, 'a legacy WS(S) Codex Base URL is saved as HTTP(S)');
});

test('Codex Responses rows are read-only previews while other endpoints stay removable', () => {
  const current = between('<EndpointTable endpoints={savedEndpoints}>', '</EndpointTable>');
  assert.match(current, /isCodexChannel && isResponsesFamilyFormat\(ep\.apiFormat\)/);
  assert.match(current, /t\('channels\.endpoints\.readOnly'\)/);
  assert.match(current, /handleRequestRemoveEndpoint\(ep\.apiFormat\)/);
});

test('disabling WebSocket is blocked only when it starves a mini model policy', () => {
  const handler = between('const handleCodexWebSocketChange = useCallback', '[modelsStarvedByRemoving]');
  assert.match(handler, /modelsStarvedByRemoving\(OPENAI_RESPONSES_WEBSOCKET\)/);
  assert.match(handler, /if \(models\.length > 0\)/);
  assert.match(handler, /setBlockedEndpointRemoval\(\{ apiFormat: OPENAI_RESPONSES_WEBSOCKET, models \}\)/);
  assert.match(handler, /setCodexResponsesWebSocket\(checked\)/);
});

test('recognizes WebSocket Responses endpoints instead of flagging them unsupported', () => {
  assert.match(source, /const RECOGNIZED_API_FORMATS: readonly string\[\] = \[\.\.\.ALLOWED_API_FORMATS, OPENAI_RESPONSES_WEBSOCKET\]/);
  assert.match(source, /!readOnly && !RECOGNIZED_API_FORMATS\.includes\(ep\.apiFormat\)/);
  const save = between('const handleSave = useCallback', 'const handleKeyDown = useCallback');
  assert.match(save, /!RECOGNIZED_API_FORMATS\.includes\(format\)/);
});

test('detail view and labels recognize the WebSocket Responses format', () => {
  const expanded = readFileSync(new URL('./channel-expanded-row.tsx', import.meta.url), 'utf8');
  assert.match(expanded, /config\?\.apiFormat === 'openai\/responses' \? getEffectiveResponsesFormats\(channel\) : \[\]/);
  assert.match(expanded, /responsesFormats\.map\(getApiFormatLabel\)\.join\(', '\)/);
  for (const locale of ['en', 'zh-CN']) {
    const messages = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/channels.json`, import.meta.url), 'utf8'));
    assert.ok(messages['channels.dialogs.fields.apiFormat.formats.openai/responses-ws'], `${locale} WebSocket format label missing`);
  }
});
