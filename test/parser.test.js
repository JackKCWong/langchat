'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { parseChatFile } = require('../src/parser.js');
const {
  SystemMessage,
  HumanMessage,
  AIMessage,
} = require('@langchain/core/messages');

test('parses a single #!system block', () => {
  const md = `#!system

You are a helpful assistant.
`;
  const messages = parseChatFile(md);
  assert.equal(messages.length, 1);
  assert.ok(messages[0] instanceof SystemMessage);
  assert.equal(messages[0].content, 'You are a helpful assistant.');
});

test('parses #!system + #!user in order', () => {
  const md = `#!system

You are a help assistant.

#!user

What's the weather like today.
`;
  const messages = parseChatFile(md);
  assert.equal(messages.length, 2);
  assert.ok(messages[0] instanceof SystemMessage);
  assert.equal(messages[0].content, 'You are a help assistant.');
  assert.ok(messages[1] instanceof HumanMessage);
  assert.equal(messages[1].content, "What's the weather like today.");
});

test('allows multiple #!user blocks in source order', () => {
  const md = `#!user

First message.

#!assistant

Reply one.

#!user

Second message.
`;
  const messages = parseChatFile(md);
  assert.equal(messages.length, 3);
  assert.ok(messages[0] instanceof HumanMessage);
  assert.equal(messages[0].content, 'First message.');
  assert.ok(messages[1] instanceof AIMessage);
  assert.equal(messages[1].content, 'Reply one.');
  assert.ok(messages[2] instanceof HumanMessage);
  assert.equal(messages[2].content, 'Second message.');
});

test('emits empty blocks as empty messages', () => {
  const md = `#!system



#!user

Hello.
`;
  const messages = parseChatFile(md);
  assert.equal(messages.length, 2);
  assert.ok(messages[0] instanceof SystemMessage);
  assert.equal(messages[0].content, '');
  assert.ok(messages[1] instanceof HumanMessage);
  assert.equal(messages[1].content, 'Hello.');
});

test('preserves multi-line content inside a block', () => {
  const md = `#!user

line one
line two
line three
`;
  const messages = parseChatFile(md);
  assert.equal(messages.length, 1);
  assert.equal(messages[0].content, 'line one\nline two\nline three');
});

test('handles headers with surrounding whitespace', () => {
  const md = `#!user   \n\nHi.\n`;
  const messages = parseChatFile(md);
  assert.equal(messages.length, 1);
  assert.equal(messages[0].content, 'Hi.');
});

test('throws on unknown role with line number', () => {
  const md = `#!foo

bar
`;
  assert.throws(
    () => parseChatFile(md),
    /Unknown role "#!foo" at line 1/
  );
});

test('throws when content appears before any header', () => {
  const md = `hello world
`;
  assert.throws(
    () => parseChatFile(md),
    /Unexpected content at line 1/
  );
});

test('throws when the file has no messages', () => {
  assert.throws(() => parseChatFile(''));
  assert.throws(() => parseChatFile('\n\n\n'));
});

test('rejects non-string input', () => {
  assert.throws(() => parseChatFile(null), TypeError);
  assert.throws(() => parseChatFile(123), TypeError);
});