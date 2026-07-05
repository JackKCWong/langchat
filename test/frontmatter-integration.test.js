'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { parseChatFile } = require('../src/parser.js');
const { parseFrontmatter } = require('../src/frontmatter.js');

test('frontmatter + parser: header is stripped and parser sees only the body', () => {
  const md = [
    '---',
    'model: qwen-vl-plus',
    'temperature: 0.3',
    '---',
    '# !system',
    '',
    'You are a help assistant.',
    '',
    '# !user',
    '',
    'Hi.',
    '',
  ].join('\n');
  const { body, opts, headerLines } = parseFrontmatter(md);
  assert.deepEqual(opts, { model: 'qwen-vl-plus', temperature: 0.3 });
  assert.equal(headerLines, 4);
  const { messages, outputSchema } = parseChatFile(body, [], {
    lineOffset: headerLines,
  });
  assert.equal(outputSchema, null);
  assert.equal(messages.length, 2);
  assert.equal(messages[1].content, 'Hi.');
});

test('frontmatter + parser: error line numbers point at the original file', () => {
  const md = [
    '---',
    'model: x',
    '---',
    '# !system',
    '',
    'sys',
    '',
    '# !foo',
    '',
    'bar',
    '',
  ].join('\n');
  const { body, headerLines } = parseFrontmatter(md);
  assert.throws(
    () => parseChatFile(body, [], { lineOffset: headerLines }),
    /Unknown role "# !foo" at line 8/
  );
});

test('frontmatter + parser: empty # !output error uses original line numbers', () => {
  const md = [
    '---',
    'model: x',
    '---',
    '# !user',
    '',
    'hi',
    '',
    '# !output',
    '',
    '',
  ].join('\n');
  const { body, headerLines } = parseFrontmatter(md);
  assert.throws(
    () => parseChatFile(body, [], { lineOffset: headerLines }),
    /# !output block at line 8 is empty/
  );
});

test('frontmatter + parser: file with no frontmatter parses unchanged with offset 0', () => {
  const md = ['# !user', '', 'hi', ''].join('\n');
  const { body, headerLines } = parseFrontmatter(md);
  assert.equal(headerLines, 0);
  assert.equal(body, md);
  const { messages } = parseChatFile(body, [], { lineOffset: headerLines });
  assert.equal(messages.length, 1);
  assert.equal(messages[0].content, 'hi');
});

test('frontmatter + parser: spec/mvp3 example file (real fixture) parses cleanly', () => {
  const md = [
    '---',
    'model: qwen-vl-plus',
    '---',
    '',
    '# !system',
    '',
    'You are a help assistant.',
    '',
    '# !user',
    '',
    'Answer my questions based on the below context:',
    '',
    '{{ include "Goku.png" }}',
    '',
    '# !user',
    '',
    'What is Sun Goku saying?',
    '',
  ].join('\n');
  const { body, opts, headerLines } = parseFrontmatter(md);
  assert.deepEqual(opts, { model: 'qwen-vl-plus' });
  const att = [{ type: 'image', mimeType: 'image/png', data: 'A', source: 'Goku.png' }];
  const { messages, outputSchema } = parseChatFile(body, att, { lineOffset: headerLines });
  assert.equal(outputSchema, null);
  assert.equal(messages.length, 3);
  assert.ok(Array.isArray(messages[1].content));
});