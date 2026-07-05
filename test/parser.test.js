'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { parseChatFile } = require('../src/parser.js');
const {
  SystemMessage,
  HumanMessage,
  AIMessage,
} = require('@langchain/core/messages');

function parse(md, att) {
  return parseChatFile(md, att).messages;
}

test('parses a single # !system block', () => {
  const md = `# !system

You are a helpful assistant.
`;
  const messages = parse(md);
  assert.equal(messages.length, 1);
  assert.ok(messages[0] instanceof SystemMessage);
  assert.equal(messages[0].content, 'You are a helpful assistant.');
});

test('parses # !system + # !user in order', () => {
  const md = `# !system

You are a help assistant.

# !user

What's the weather like today.
`;
  const messages = parse(md);
  assert.equal(messages.length, 2);
  assert.ok(messages[0] instanceof SystemMessage);
  assert.equal(messages[0].content, 'You are a help assistant.');
  assert.ok(messages[1] instanceof HumanMessage);
  assert.equal(messages[1].content, "What's the weather like today.");
});

test('allows multiple # !user blocks in source order', () => {
  const md = `# !user

First message.

# !assistant

Reply one.

# !user

Second message.
`;
  const messages = parse(md);
  assert.equal(messages.length, 3);
  assert.ok(messages[0] instanceof HumanMessage);
  assert.equal(messages[0].content, 'First message.');
  assert.ok(messages[1] instanceof AIMessage);
  assert.equal(messages[1].content, 'Reply one.');
  assert.ok(messages[2] instanceof HumanMessage);
  assert.equal(messages[2].content, 'Second message.');
});

test('emits empty blocks as empty messages', () => {
  const md = `# !system


# !user

Hello.
`;
  const messages = parse(md);
  assert.equal(messages.length, 2);
  assert.ok(messages[0] instanceof SystemMessage);
  assert.equal(messages[0].content, '');
  assert.ok(messages[1] instanceof HumanMessage);
  assert.equal(messages[1].content, 'Hello.');
});

test('preserves multi-line content inside a block', () => {
  const md = `# !user

line one
line two
line three
`;
  const messages = parse(md);
  assert.equal(messages.length, 1);
  assert.equal(messages[0].content, 'line one\nline two\nline three');
});

test('handles headers with surrounding whitespace', () => {
  const md = `# !user   \n\nHi.\n`;
  const messages = parse(md);
  assert.equal(messages.length, 1);
  assert.equal(messages[0].content, 'Hi.');
});

test('throws on unknown role with line number', () => {
  const md = `# !foo

bar
`;
  assert.throws(
    () => parseChatFile(md),
    /Unknown role "# !foo" at line 1/
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
  const md = `# !user\n\nLook: {{ include "a.png" }}\n`;
  const att = [{ type: 'image', mimeType: 'image/png', data: 'AAAA', source: 'a.png' }];
  const messages = parse(md, att);
  assert.equal(messages.length, 1);
  assert.ok(Array.isArray(messages[0].content));
  assert.equal(messages[0].content.length, 2);
  assert.equal(messages[0].content[0].type, 'text');
  assert.equal(messages[0].content[0].text, 'Look: ');
  assert.equal(messages[0].content[1].type, 'image');
  assert.equal(messages[0].content[1].data, 'AAAA');
});

test('text-only user messages stay as strings when attachments exist', () => {
  const md = `# !user\n\njust text, no directive\n# !user\n\nalso just text\n`;
  const messages = parse(md, []);
  assert.equal(messages.length, 2);
  assert.equal(typeof messages[0].content, 'string');
  assert.equal(typeof messages[1].content, 'string');
});

test('image directive in a # !system block throws with a line number', () => {
  const md = `# !system\n\nYou see {{ include "a.png" }}\n`;
  const att = [{ type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' }];
  assert.throws(
    () => parseChatFile(md, att),
    /image include.*at line 1.*only supported.*# !user/
  );
});

test('image directive in a # !assistant block throws', () => {
  const md = `# !assistant\n\nI see {{ include "a.png" }}\n`;
  const att = [{ type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' }];
  assert.throws(
    () => parseChatFile(md, att),
    /image include.*at line 1.*only supported.*# !user/
  );
});

test('throws when directives outnumber attachments', () => {
  const md = `# !user\n\n{{ include "a.png" }} and {{ include "b.jpg" }}\n`;
  const att = [{ type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' }];
  assert.throws(
    () => parseChatFile(md, att),
    /directives exceed attachments/
  );
});

test('throws when attachments outnumber directives', () => {
  const md = `# !user\n\nplain text no directive\n`;
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
  const md = `# !user\n\nfirst {{ include "a.png" }} second {{ include "b.jpg" }} end\n`;
  const att = [
    { type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' },
    { type: 'image', mimeType: 'image/jpeg', data: 'B', source: 'b.jpg' },
  ];
  const messages = parse(md, att);
  assert.ok(Array.isArray(messages[0].content));
  assert.deepEqual(messages[0].content, [
    { type: 'text', text: 'first ' },
    { type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' },
    { type: 'text', text: ' second ' },
    { type: 'image', mimeType: 'image/jpeg', data: 'B', source: 'b.jpg' },
    { type: 'text', text: ' end' },
  ]);
});

test('multiple include directives across multiple user messages consume attachments in order', () => {
  const md = `# !user\n\nfirst {{ include "a.png" }}\n# !assistant\n\nok\n# !user\n\nthen {{ include "b.jpg" }} and {{ include "c.gif" }}\n`;
  const att = [
    { type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' },
    { type: 'image', mimeType: 'image/jpeg', data: 'B', source: 'b.jpg' },
    { type: 'image', mimeType: 'image/gif', data: 'C', source: 'c.gif' },
  ];
  const messages = parse(md, att);
  assert.equal(messages.length, 3);
  assert.ok(Array.isArray(messages[0].content));
  assert.deepEqual(messages[0].content, [
    { type: 'text', text: 'first ' },
    { type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' },
  ]);
  assert.equal(messages[1].content, 'ok');
  assert.ok(Array.isArray(messages[2].content));
  assert.deepEqual(messages[2].content, [
    { type: 'text', text: 'then ' },
    { type: 'image', mimeType: 'image/jpeg', data: 'B', source: 'b.jpg' },
    { type: 'text', text: ' and ' },
    { type: 'image', mimeType: 'image/gif', data: 'C', source: 'c.gif' },
  ]);
});

test('a single user message mixes text segments and include directives preserving order', () => {
  const md = `# !user\n\nCompare these:\n1) {{ include "a.png" }}\n2) {{ include "b.jpg" }}\n3) {{ include "c.webp" }}\nWhich differ?\n`;
  const att = [
    { type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' },
    { type: 'image', mimeType: 'image/jpeg', data: 'B', source: 'b.jpg' },
    { type: 'image', mimeType: 'image/webp', data: 'C', source: 'c.webp' },
  ];
  const messages = parse(md, att);
  assert.equal(messages.length, 1);
  const blocks = messages[0].content;
  const flattened = blocks.map((b) =>
    b.type === 'text' ? `T:${b.text}` : `I:${b.mimeType}`
  );
  assert.deepEqual(flattened, [
    'T:Compare these:\n1) ',
    'I:image/png',
    'T:\n2) ',
    'I:image/jpeg',
    'T:\n3) ',
    'I:image/webp',
    'T:\nWhich differ?',
  ]);
  assert.deepEqual(blocks[1], { type: 'image', mimeType: 'image/png', data: 'A', source: 'a.png' });
  assert.deepEqual(blocks[3], { type: 'image', mimeType: 'image/jpeg', data: 'B', source: 'b.jpg' });
  assert.deepEqual(blocks[5], { type: 'image', mimeType: 'image/webp', data: 'C', source: 'c.webp' });
});

test('a single user message mixes text files and images across multiple include directives', () => {
  const md = `# !user\n\nLog: {{ include "run.log" }}\nScreenshot: {{ include "shot.png" }}\nConfig: {{ include "config.yaml" }}\n`;
  const att = [
    { type: 'text', text: 'INFO boot ok', source: 'run.log' },
    { type: 'image', mimeType: 'image/png', data: 'SHOT', source: 'shot.png' },
    { type: 'text', text: 'port: 8080', source: 'config.yaml' },
  ];
  const messages = parse(md, att);
  assert.equal(messages.length, 1);
  assert.ok(Array.isArray(messages[0].content));
  assert.deepEqual(messages[0].content, [
    { type: 'text', text: 'Log: ' },
    { type: 'text', text: 'INFO boot ok', source: 'run.log' },
    { type: 'text', text: '\nScreenshot: ' },
    { type: 'image', mimeType: 'image/png', data: 'SHOT', source: 'shot.png' },
    { type: 'text', text: '\nConfig: ' },
    { type: 'text', text: 'port: 8080', source: 'config.yaml' },
  ]);
});

test('rejects non-array attachments', () => {
  assert.throws(() => parseChatFile('# !user\n\nx\n', 'not-an-array'), TypeError);
});

test('returns outputSchema: null when no # !output block is present', () => {
  const md = `# !system\n\nsys\n# !user\n\nhi\n`;
  const { messages, outputSchema } = parseChatFile(md);
  assert.equal(messages.length, 2);
  assert.equal(outputSchema, null);
});

test('# !output with fenced JSON returns parsed schema and adds no message', () => {
  const md = `# !system\n\nsys\n# !user\n\nhi\n# !output\n\n\`\`\`json\n{"type":"object","properties":{"name":{"type":"string"}}}\n\`\`\`\n`;
  const { messages, outputSchema } = parseChatFile(md);
  assert.equal(messages.length, 2);
  assert.deepEqual(outputSchema, {
    type: 'object',
    properties: { name: { type: 'string' } },
  });
});

test('# !output with a plain (unfenced) JSON object is accepted', () => {
  const md = `# !user\n\nhi\n# !output\n\n{"type":"object"}\n`;
  const { messages, outputSchema } = parseChatFile(md);
  assert.equal(messages.length, 1);
  assert.deepEqual(outputSchema, { type: 'object' });
});

test('# !output accepts a fence without a language tag', () => {
  const md = `# !user\n\nhi\n# !output\n\n\`\`\`\n{"a":1}\n\`\`\`\n`;
  const { messages, outputSchema } = parseChatFile(md);
  assert.equal(messages.length, 1);
  assert.deepEqual(outputSchema, { a: 1 });
});

test('multiple # !output blocks throw with a line number', () => {
  const md = `# !user\n\nhi\n# !output\n\n{"a":1}\n# !output\n\n{"b":2}\n`;
  assert.throws(
    () => parseChatFile(md),
    /duplicate # !output block at line \d+/
  );
});

test('empty # !output block throws with the header line number', () => {
  const md = `# !user\n\nhi\n# !output\n\n\n`;
  assert.throws(
    () => parseChatFile(md),
    /# !output block at line 4 is empty/
  );
});

test('invalid JSON in # !output throws with the header line number', () => {
  const md = `# !user\n\nhi\n# !output\n\n\`\`\`json\n{ not json }\n\`\`\`\n`;
  assert.throws(
    () => parseChatFile(md),
    /# !output block at line 4 is not valid JSON/
  );
});

test('messages and outputSchema are independent: a chat with only # !output still fails the "no messages" check', () => {
  const md = `# !output\n\n{"type":"object"}\n`;
  assert.throws(
    () => parseChatFile(md),
    /No messages found/
  );
});