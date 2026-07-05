'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { resolveIncludes } = require('../src/includes.js');

function makeTmpDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'langchat-includes-'));
}

function writeFile(dir, name, contents) {
  const p = path.join(dir, name);
  fs.writeFileSync(p, contents);
  return p;
}

test('expands a simple include next to the chat file', () => {
  const dir = makeTmpDir();
  writeFile(dir, 'chat.md', 'before {{ include "snippet.txt" }} after');
  writeFile(dir, 'snippet.txt', 'HELLO');
  const out = resolveIncludes(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
    baseDir: dir,
  });
  assert.equal(out, 'before HELLO after');
});

test('expands nested includes recursively', () => {
  const dir = makeTmpDir();
  writeFile(dir, 'chat.md', '{{ include "level1.md" }}');
  writeFile(dir, 'level1.md', 'L1 {{ include "leaf.txt" }} L1');
  writeFile(dir, 'leaf.txt', 'LEAF');
  const out = resolveIncludes(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
    baseDir: dir,
  });
  assert.equal(out, 'L1 LEAF L1');
});

test('allows siblings to share an include (diamond shape)', () => {
  const dir = makeTmpDir();
  writeFile(
    dir,
    'chat.md',
    '{{ include "a.md" }} || {{ include "b.md" }}'
  );
  writeFile(dir, 'a.md', '[A:{{ include "shared.txt" }}]');
  writeFile(dir, 'b.md', '[B:{{ include "shared.txt" }}]');
  writeFile(dir, 'shared.txt', 'S');
  const out = resolveIncludes(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
    baseDir: dir,
  });
  assert.equal(out, '[A:S] || [B:S]');
});

test('detects cyclic includes', () => {
  const dir = makeTmpDir();
  writeFile(dir, 'a.md', '{{ include "b.md" }}');
  writeFile(dir, 'b.md', '{{ include "a.md" }}');
  assert.throws(
    () =>
      resolveIncludes('{{ include "a.md" }}', { baseDir: dir }),
    /cyclic include detected/
  );
});

test('errors with a useful message when an included file is missing', () => {
  const dir = makeTmpDir();
  writeFile(dir, 'chat.md', '{{ include "nope.txt" }}');
  assert.throws(
    () =>
      resolveIncludes(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
        baseDir: dir,
      }),
    /include failed: nope\.txt/
  );
});

test('blocks path traversal by default', () => {
  const dir = makeTmpDir();
  const outside = makeTmpDir();
  writeFile(outside, 'secret.txt', 'SECRET');
  writeFile(dir, 'chat.md', '{{ include "../secret.txt" }}');
  assert.throws(
    () =>
      resolveIncludes(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
        baseDir: dir,
      }),
    /escapes base directory/
  );
});

test('permits escape when allowEscape is true', () => {
  const parent = makeTmpDir();
  const dir = path.join(parent, 'inner');
  const outside = path.join(parent, 'outside');
  fs.mkdirSync(dir);
  fs.mkdirSync(outside);
  writeFile(outside, 'secret.txt', 'SECRET');
  writeFile(dir, 'chat.md', '{{ include "../outside/secret.txt" }}');
  const out = resolveIncludes(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
    baseDir: dir,
    allowEscape: true,
  });
  assert.equal(out, 'SECRET');
});

test('resolves absolute paths against the filesystem root', () => {
  const dir = makeTmpDir();
  const target = path.join(dir, 'abs.txt');
  fs.writeFileSync(target, 'ABS');
  writeFile(dir, 'chat.md', `{{ include "${target.replace(/\\/g, '\\\\')}" }}`);
  const out = resolveIncludes(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
    baseDir: dir,
  });
  assert.equal(out, 'ABS');
});

test('respects maxDepth', () => {
  const dir = makeTmpDir();
  let prev = 'leaf.txt';
  fs.writeFileSync(path.join(dir, 'leaf.txt'), 'BOTTOM');
  for (let i = 0; i < 10; i++) {
    const next = `level${i}.md`;
    fs.writeFileSync(
      path.join(dir, next),
      `{{ include "${prev}" }}`
    );
    prev = next;
  }
  assert.throws(
    () =>
      resolveIncludes(`{{ include "${prev}" }}`, {
        baseDir: dir,
        maxDepth: 3,
      }),
    /include depth exceeded 3/
  );
});

test('accepts loose whitespace inside the directive', () => {
  const dir = makeTmpDir();
  writeFile(dir, 'a.txt', 'A');
  writeFile(dir, 'b.txt', 'B');
  writeFile(dir, 'c.txt', 'C');
  const md = [
    '{{include "a.txt"}}',
    '{{  include  "b.txt"  }}',
    '{{\ninclude "c.txt"\n}}',
  ].join('|');
  const out = resolveIncludes(md, { baseDir: dir });
  assert.equal(out, 'A|B|C');
});

test('leaves text without directives unchanged', () => {
  const dir = makeTmpDir();
  const md = 'plain text {{ not include }} {{ include }} {{include "x"}}';
  writeFile(dir, 'x', 'X');
  // The last one resolves; the rest are ignored (no match or unterminated).
  const out = resolveIncludes(md, { baseDir: dir });
  assert.equal(out, 'plain text {{ not include }} {{ include }} X');
});

test('expansion integrates with the chat parser (directive inside a #!user block)', () => {
  const dir = makeTmpDir();
  writeFile(dir, 'chat.md', `#!user\n\nAnswer based on: {{ include "ctx.txt" }}\n`);
  writeFile(dir, 'ctx.txt', 'the context');
  const { parseChatFile } = require('../src/parser.js');
  const expanded = resolveIncludes(
    fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
    { baseDir: dir }
  );
  const messages = parseChatFile(expanded);
  assert.equal(messages.length, 1);
  assert.equal(messages[0].content, 'Answer based on: the context');
});

test('rejects non-string input', () => {
  assert.throws(() => resolveIncludes(null), TypeError);
  assert.throws(() => resolveIncludes(42), TypeError);
});