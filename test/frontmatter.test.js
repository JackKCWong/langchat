'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { parseFrontmatter, parseScalarValue } = require('../src/frontmatter.js');

test('returns empty opts and original text when no frontmatter is present', () => {
  const md = '# !system\n\nhi\n';
  const r = parseFrontmatter(md);
  assert.deepEqual(r.opts, {});
  assert.equal(r.headerLines, 0);
  assert.equal(r.body, md);
});

test('returns empty opts when file starts with a non-delimiter line', () => {
  const md = '   ---\nmodel: x\n---\nbody';
  const r = parseFrontmatter(md);
  assert.equal(r.headerLines, 0);
  assert.equal(r.body, md);
});

test('returns empty opts when file is just a lone "---"', () => {
  const r = parseFrontmatter('---');
  assert.equal(r.headerLines, 0);
  assert.equal(r.body, '---');
});

test('parses a single string key', () => {
  const md = '---\nmodel: qwen-vl-plus\n---\n# !user\n\nhi\n';
  const r = parseFrontmatter(md);
  assert.deepEqual(r.opts, { model: 'qwen-vl-plus' });
  assert.equal(r.headerLines, 3);
  assert.equal(r.body, '# !user\n\nhi\n');
});

test('parses multiple keys of mixed types', () => {
  const md = [
    '---',
    'model: qwen-vl-plus',
    'streaming: true',
    'temperature: 0.7',
    'max_tokens: 1024',
    'thinking: false',
    '---',
    '# !user',
    '',
    'hi',
    '',
  ].join('\n');
  const r = parseFrontmatter(md);
  assert.deepEqual(r.opts, {
    model: 'qwen-vl-plus',
    streaming: true,
    temperature: 0.7,
    max_tokens: 1024,
    thinking: false,
  });
  assert.equal(r.headerLines, 7);
  assert.equal(r.body, '# !user\n\nhi\n');
});

test('parses integers vs floats distinctly', () => {
  const r = parseFrontmatter('---\na: 1\nb: 1.0\nc: -3\nd: 0.25\n---\n');
  assert.equal(r.opts.a, 1);
  assert.equal(r.opts.b, 1);
  assert.equal(r.opts.c, -3);
  assert.equal(r.opts.d, 0.25);
  assert.equal(typeof r.opts.a, 'number');
  assert.equal(typeof r.opts.b, 'number');
});

test('parses double- and single-quoted strings', () => {
  const r = parseFrontmatter('---\na: "hello world"\nb: \'with: colon\'\n---\n');
  assert.equal(r.opts.a, 'hello world');
  assert.equal(r.opts.b, 'with: colon');
});

test('parses null and empty values', () => {
  const r = parseFrontmatter('---\na: null\nb: ~\nc: \n---\n');
  assert.equal(r.opts.a, null);
  assert.equal(r.opts.b, null);
  assert.equal(r.opts.c, '');
});

test('skips blank lines and # comments inside the header', () => {
  const r = parseFrontmatter('---\n# a comment\n\nmodel: foo\n# trailing\n---\n');
  assert.deepEqual(r.opts, { model: 'foo' });
});

test('handles \\r\\n line endings without mangling the body', () => {
  const md = '---\r\nmodel: x\r\n---\r\n# !user\r\n\r\nhi\r\n';
  const r = parseFrontmatter(md);
  assert.deepEqual(r.opts, { model: 'x' });
  assert.equal(r.headerLines, 3);
  assert.ok(r.body.startsWith('# !user'));
  assert.ok(r.body.includes('hi'));
});

test('treats a lone "---" without a closer as no frontmatter', () => {
  const md = '---\nmodel: x\n# !user\n\nhi\n';
  const r = parseFrontmatter(md);
  assert.equal(r.headerLines, 0);
  assert.equal(r.body, md);
});

test('throws on a line without ":"', () => {
  assert.throws(
    () => parseFrontmatter('---\nbareword\n---\n'),
    /expected "key: value" but got "bareword"/
  );
});

test('throws on empty key', () => {
  assert.throws(
    () => parseFrontmatter('---\n: value\n---\n'),
    /empty key/
  );
});

test('throws on invalid key characters', () => {
  assert.throws(
    () => parseFrontmatter('---\n1bad: x\n---\n'),
    /invalid key "1bad"/
  );
});

test('throws on indented key', () => {
  assert.throws(
    () => parseFrontmatter('---\n  model: x\n---\n'),
    /keys must not be indented/
  );
});

test('rejects non-string input', () => {
  assert.throws(() => parseFrontmatter(null), TypeError);
  assert.throws(() => parseFrontmatter(42), TypeError);
});

test('parseScalarValue handles the value subset directly', () => {
  assert.equal(parseScalarValue('foo'), 'foo');
  assert.equal(parseScalarValue('  foo  '), 'foo');
  assert.equal(parseScalarValue('"with spaces"'), 'with spaces');
  assert.equal(parseScalarValue("'single quoted'"), 'single quoted');
  assert.equal(parseScalarValue('true'), true);
  assert.equal(parseScalarValue('false'), false);
  assert.equal(parseScalarValue('null'), null);
  assert.equal(parseScalarValue('~'), null);
  assert.equal(parseScalarValue(''), '');
  assert.equal(parseScalarValue('3'), 3);
  assert.equal(parseScalarValue('3.14'), 3.14);
  assert.equal(parseScalarValue('-2'), -2);
});