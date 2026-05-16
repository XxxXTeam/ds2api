'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

async function loadUtils() {
  return import('../../webui/src/features/chatHistory/chatHistoryUtils.js');
}

test('chat history strict parser merges current input file placeholder', async () => {
  const {
    buildListModeMessages,
  } = await loadUtils();
  const t = (key) => key;
  const item = {
    messages: [{
      role: 'user',
      content: 'Продолжай с последнего состояния из приложенного контекста DS2API_HISTORY.txt. Считай его текущим рабочим состоянием и напрямую отвечай на последний запрос пользователя на китайском языке.',
    }],
    history_text: [
      '<|begin|of|sentence|>',
      '<|User|>hello',
      '<|Assistant|>hi<|end|of|sentence|>',
    ].join(''),
  };

  const result = buildListModeMessages(item, t);
  assert.equal(result.historyMerged, true);
  assert.deepEqual(result.messages, [
    { role: 'user', content: 'hello' },
    { role: 'assistant', content: 'hi' },
  ]);
});

test('chat history strict parser inserts history after system messages', async () => {
  const {
    buildListModeMessages,
  } = await loadUtils();
  const t = (key) => key;
  const item = {
    messages: [
      { role: 'system', content: 'policy' },
      { role: 'user', content: 'latest' },
    ],
    history_text: [
      '<|begin|of|sentence|>',
      '<|User|>old',
      '<|Assistant|>done<|end|of|sentence|>',
    ].join(''),
  };

  const result = buildListModeMessages(item, t);
  assert.equal(result.historyMerged, true);
  assert.deepEqual(result.messages, [
    { role: 'system', content: 'policy' },
    { role: 'user', content: 'old' },
    { role: 'assistant', content: 'done' },
    { role: 'user', content: 'latest' },
  ]);
});

test('chat history transcript parser replaces current input file placeholder', async () => {
  const {
    buildListModeMessages,
  } = await loadUtils();
  const t = (key) => key;
  const item = {
    messages: [{
      role: 'user',
      content: 'Продолжай с последнего состояния из приложенного контекста DS2API_HISTORY.txt. Считай его текущим рабочим состоянием и напрямую отвечай на последний запрос пользователя на китайском языке.',
    }],
    history_text: [
      '# DS2API_HISTORY.txt',
      'Предыдущая история диалога и ход выполнения инструментов.',
      '',
      '=== 1. SYSTEM ===',
      'policy',
      '',
      '=== 2. USER ===',
      'hello',
      '',
      '=== 3. ASSISTANT ===',
      'hi',
      '',
      '=== 4. TOOL ===',
      '[name=search_web tool_call_id=call_1]',
      '{"ok":true}',
      '',
      '=== 5. USER ===',
      'latest',
      '',
    ].join('\n'),
  };

  const result = buildListModeMessages(item, t);
  assert.equal(result.historyMerged, true);
  assert.deepEqual(result.messages, [
    { role: 'system', content: 'policy' },
    { role: 'user', content: 'hello' },
    { role: 'assistant', content: 'hi' },
    { role: 'tool', content: '[name=search_web tool_call_id=call_1]\n{"ok":true}' },
    { role: 'user', content: 'latest' },
  ]);
});
