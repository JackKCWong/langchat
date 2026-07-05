'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { parseArgs } = require('../src/cli.js');

test('empty argv returns no file, no flags', () => {
  const opts = parseArgs([]);
  assert.equal(opts.file, null);
  assert.equal(opts.stream, false);
  assert.equal(opts.help, false);
});

test('-h sets help', () => {
  const opts = parseArgs(['-h']);
  assert.equal(opts.help, true);
  assert.equal(opts.stream, false);
  assert.equal(opts.file, null);
});

test('--help sets help', () => {
  const opts = parseArgs(['--help']);
  assert.equal(opts.help, true);
});

test('-s sets stream', () => {
  const opts = parseArgs(['-s']);
  assert.equal(opts.stream, true);
  assert.equal(opts.file, null);
  assert.equal(opts.help, false);
});

test('--stream sets stream', () => {
  const opts = parseArgs(['--stream']);
  assert.equal(opts.stream, true);
});

test('-s with a file', () => {
  const opts = parseArgs(['-s', 'chat.md']);
  assert.equal(opts.stream, true);
  assert.equal(opts.file, 'chat.md');
});

test('--stream with a file', () => {
  const opts = parseArgs(['--stream', 'chat.md']);
  assert.equal(opts.stream, true);
  assert.equal(opts.file, 'chat.md');
});

test('file then flag (order does not matter)', () => {
  const opts = parseArgs(['chat.md', '-s']);
  assert.equal(opts.stream, true);
  assert.equal(opts.file, 'chat.md');
});

test('bare file argument', () => {
  const opts = parseArgs(['chat.md']);
  assert.equal(opts.file, 'chat.md');
  assert.equal(opts.stream, false);
  assert.equal(opts.help, false);
});

test('multiple files throw', () => {
  assert.throws(
    () => parseArgs(['a.md', 'b.md']),
    /Expected exactly one <chat\.md> argument/
  );
});

test('unknown long flag throws', () => {
  assert.throws(() => parseArgs(['--bogus']), /Unknown option: --bogus/);
});

test('unknown short flag throws', () => {
  assert.throws(() => parseArgs(['-x']), /Unknown option: -x/);
});

test('-- terminates option parsing', () => {
  const opts = parseArgs(['--', 'chat.md']);
  assert.equal(opts.file, 'chat.md');
  assert.equal(opts.stream, false);
});

test('-- makes a flag-shaped name a positional', () => {
  const opts = parseArgs(['--', '-s']);
  assert.equal(opts.file, '-s');
  assert.equal(opts.stream, false);
});

test('-h with file: help wins', () => {
  const opts = parseArgs(['-h', 'chat.md']);
  assert.equal(opts.help, true);
  assert.equal(opts.file, 'chat.md');
});

test('stream + help: help still recognized', () => {
  const opts = parseArgs(['-s', '--help']);
  assert.equal(opts.help, true);
  assert.equal(opts.stream, true);
});