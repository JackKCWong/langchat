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

test('user message with one image directive becomes a content array', () => {
  const md = `#!user\n\nLook: {{ include "a.png" }}\n`;
  const att = [{ type: 'image', mimeType: 'image/png', data: 'AAAA', source: 'a.png' }];
  const messages = parseChatFile(md, att);
  assert.equal(messages.length, 1);
  assert.ok(Array.isArray(messages[0].content));
  assert.equal(messages[0].content.length, 2);
  assert.equal(messages[0].content[0].type, 'text');
  assert.equal(messages[0].content[0].text, 'Look: ');
  assert.equal(messages[0].content[1].type, 'image');
  assert.equal(messages[0].content[1].data, 'AAAA');
});

test('text-only user messages stay as strings when attachments exist', () => {
  const md = `#!user\n\njust text, no directive\n#!user\n\nalso just text\n`;
  const messages = parseChatFile(md, []);
  assert.equal(messages.length, 2);
  assert.equal(typeof messages[0].content, 'string');
  assert.equal(typeof messages[1].content, 'string');
});

test('image directive in a #!system block throws with a line number', () => {
  const md = `#!system\n\nYou see {{ include "a.png" }}\n`;
  const att = [{ type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' }];
  assert.throws(
    () => parseChatFile(md, att),
    /image include.*at line 1.*only supported.*#!user/
  );
});

test('image directive in a #!assistant block throws', () => {
  const md = `#!assistant\n\nI see {{ include "a.png" }}\n`;
  const att = [{ type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' }];
  assert.throws(
    () => parseChatFile(md, att),
    /image include.*at line 1.*only supported.*#!user/
  );
});

test('throws when directives outnumber attachments', () => {
  const md = `#!user\n\n{{ include "a.png" }} and {{ include "b.jpg" }}\n`;
  const att = [{ type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' }];
  assert.throws(
    () => parseChatFile(md, att),
    /directives exceed attachments/
  );
});

test('throws when attachments outnumber directives', () => {
  const md = `#!user\n\nplain text no directive\n`;
  const att = [
    { type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' },
    { type: 'image', mimeType: 'image/jpeg', data: 'B', source: 'b.jpg' },
  ];
  assert.throws(
    () => parseChatFile(md, att),
    /attachment count mismatch/
  );
});

test('multiple images in one user message preserve directive order', () => {
  const md = `#!user\n\nfirst {{ include "a.png" }} second {{ include "b.jpg" }} end\n`;
  const att = [
    { type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' },
    { type: 'image', mimeType: 'image/jpeg', data: 'B', source: 'b.jpg' },
  ];
  const messages = parseChatFile(md, att);
  assert.ok(Array.isArray(messages[0].content));
  assert.deepEqual(messages[0].content, [
    { type: 'text', text: 'first ' },
    { type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' },
    { type: 'text', text: ' second ' },
    { type: 'image', mimeType: 'image/jpeg', data: 'B', source: 'b.jpg' },
    { type: 'text', text: ' end' },
  ]);
});

test('rejects non-array attachments', () => {
  assert.throws(() => parseChatFile('#!user\n\nx\n', 'not-an-array'), TypeError);
});