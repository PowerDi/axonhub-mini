import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const sourcePath = join(import.meta.dirname, 'clipboard.ts');
const source = readFileSync(sourcePath, 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;

const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { copyTextToClipboard } = await import(moduleUrl);

// makeFakeDom installs minimal document/window/navigator globals and reports what was touched.
function makeFakeDom({ isSecureContext = true, writeText, execCommand } = {}) {
  const calls = { writeText: [], execCommand: [], appended: [], removed: [], focused: [] };

  const body = {
    appendChild(node) {
      calls.appended.push(node);
      node.isConnected = true;
      return node;
    },
  };

  const previousActive = {
    tagName: 'BUTTON',
    focus() {
      calls.focused.push('previous');
    },
  };

  const document = {
    activeElement: previousActive,
    body,
    createElement() {
      const node = {
        style: {},
        attributes: {},
        value: '',
        isConnected: false,
        setAttribute(name, value) {
          this.attributes[name] = value;
        },
        focus() {
          calls.focused.push('textarea');
        },
        select() {},
        setSelectionRange() {},
        remove() {
          calls.removed.push(this);
          this.isConnected = false;
        },
      };
      return node;
    },
  };

  if (execCommand) {
    document.execCommand = (command) => {
      calls.execCommand.push(command);
      return execCommand(command);
    };
  }

  const window = {
    isSecureContext,
    getSelection: () => ({
      rangeCount: 0,
      getRangeAt: () => null,
      removeAllRanges() {},
      addRange() {},
    }),
  };

  const navigator = {};
  if (writeText) {
    navigator.clipboard = {
      writeText: async (text) => {
        calls.writeText.push(text);
        return writeText(text);
      },
    };
  }

  defineGlobal('document', document);
  defineGlobal('window', window);
  // Node ships a getter-only globalThis.navigator, so plain assignment is not enough.
  defineGlobal('navigator', navigator);

  return calls;
}

function defineGlobal(name, value) {
  Object.defineProperty(globalThis, name, { value, writable: true, configurable: true });
}

test.afterEach(() => {
  delete globalThis.document;
  delete globalThis.window;
  delete globalThis.navigator;
});

test('uses the async clipboard API in a secure context', async () => {
  const calls = makeFakeDom({ isSecureContext: true, writeText: () => {}, execCommand: () => true });

  await copyTextToClipboard('hello');

  assert.deepEqual(calls.writeText, ['hello']);
  assert.deepEqual(calls.execCommand, []);
  assert.deepEqual(calls.appended, []);
});

test('falls back to execCommand when the context is not secure', async () => {
  const calls = makeFakeDom({ isSecureContext: false, writeText: () => {}, execCommand: () => true });

  await copyTextToClipboard('over-http');

  assert.deepEqual(calls.writeText, []);
  assert.deepEqual(calls.execCommand, ['copy']);
  assert.equal(calls.appended[0].value, 'over-http');
});

test('falls back to execCommand when writeText rejects', async () => {
  const calls = makeFakeDom({
    isSecureContext: true,
    writeText: () => {
      throw new Error('NotAllowedError');
    },
    execCommand: () => true,
  });

  await copyTextToClipboard('rejected-then-copied');

  assert.deepEqual(calls.writeText, ['rejected-then-copied']);
  assert.deepEqual(calls.execCommand, ['copy']);
});

test('rejects when execCommand reports failure', async () => {
  const calls = makeFakeDom({ isSecureContext: false, execCommand: () => false });

  await assert.rejects(() => copyTextToClipboard('nope'), /execCommand\("copy"\) failed/);
  assert.deepEqual(calls.execCommand, ['copy']);
});

test('rejects when neither the clipboard API nor execCommand is available', async () => {
  makeFakeDom({ isSecureContext: false });

  await assert.rejects(() => copyTextToClipboard('nope'), /execCommand is not supported/);
});

test('rejects when there is no DOM', async () => {
  defineGlobal('window', { isSecureContext: false });
  defineGlobal('navigator', {});

  await assert.rejects(() => copyTextToClipboard('nope'), /no DOM/);
});

test('removes the temporary node on success and on failure', async () => {
  const okCalls = makeFakeDom({ isSecureContext: false, execCommand: () => true });
  await copyTextToClipboard('ok');
  assert.equal(okCalls.appended.length, 1);
  assert.deepEqual(okCalls.removed, okCalls.appended);
  assert.equal(okCalls.appended[0].isConnected, false);

  const failCalls = makeFakeDom({ isSecureContext: false, execCommand: () => false });
  await assert.rejects(() => copyTextToClipboard('fail'));
  assert.equal(failCalls.appended.length, 1);
  assert.deepEqual(failCalls.removed, failCalls.appended);
  assert.equal(failCalls.appended[0].isConnected, false);
});

test('marks the temporary node as hidden from assistive tech and restores focus', async () => {
  const calls = makeFakeDom({ isSecureContext: false, execCommand: () => true });

  await copyTextToClipboard('a11y');

  const node = calls.appended[0];
  assert.equal(node.attributes['aria-hidden'], 'true');
  assert.equal(node.attributes.tabindex, '-1');
  assert.equal(node.style.position, 'fixed');
  assert.deepEqual(calls.focused, ['textarea', 'previous']);
});

test('restores the previous selection range', async () => {
  makeFakeDom({ isSecureContext: false, execCommand: () => true });

  const range = { id: 'previous-range' };
  const restored = [];
  let cleared = 0;
  globalThis.window.getSelection = () => ({
    rangeCount: 1,
    getRangeAt: () => range,
    removeAllRanges() {
      cleared += 1;
    },
    addRange(value) {
      restored.push(value);
    },
  });

  await copyTextToClipboard('selection');

  assert.equal(cleared, 1);
  assert.deepEqual(restored, [range]);
});
