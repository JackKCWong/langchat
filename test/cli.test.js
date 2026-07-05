'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const {
  parseArgs,
  parseThinkingValue,
  resolveOutputPath,
  writeResponse,
  createLineWriter,
  runOnce,
  runStreamed,
  extractThinking,
} = require('../src/cli.js');

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

test('--allow-include-escape defaults to false', () => {
  const opts = parseArgs(['chat.md']);
  assert.equal(opts.allowIncludeEscape, false);
});

test('--allow-include-escape sets the flag', () => {
  const opts = parseArgs(['--allow-include-escape', 'chat.md']);
  assert.equal(opts.allowIncludeEscape, true);
  assert.equal(opts.file, 'chat.md');
});

test('--allow-include-escape works with -s', () => {
  const opts = parseArgs(['-s', '--allow-include-escape', 'chat.md']);
  assert.equal(opts.stream, true);
  assert.equal(opts.allowIncludeEscape, true);
});

test('--model defaults to null', () => {
  const opts = parseArgs(['chat.md']);
  assert.equal(opts.model, null);
});

test('-m sets the model', () => {
  const opts = parseArgs(['-m', 'gpt-4o-mini', 'chat.md']);
  assert.equal(opts.model, 'gpt-4o-mini');
  assert.equal(opts.file, 'chat.md');
});

test('--model sets the model', () => {
  const opts = parseArgs(['--model', 'llama3.1', 'chat.md']);
  assert.equal(opts.model, 'llama3.1');
  assert.equal(opts.file, 'chat.md');
});

test('-m works after the file (order does not matter)', () => {
  const opts = parseArgs(['chat.md', '-m', 'deepseek-chat']);
  assert.equal(opts.model, 'deepseek-chat');
  assert.equal(opts.file, 'chat.md');
});

test('-m without a value throws', () => {
  assert.throws(() => parseArgs(['-m']), /requires a model name/);
});

test('--model without a value throws', () => {
  assert.throws(() => parseArgs(['--model']), /requires a model name/);
});

test('--model with a flag-shaped value throws', () => {
  assert.throws(() => parseArgs(['--model', '-s']), /requires a model name/);
});

test('-m combined with -s and a file', () => {
  const opts = parseArgs(['-s', '-m', 'gpt-4o', 'chat.md']);
  assert.equal(opts.stream, true);
  assert.equal(opts.model, 'gpt-4o');
  assert.equal(opts.file, 'chat.md');
});

test('--output defaults to null', () => {
  const opts = parseArgs(['chat.md']);
  assert.equal(opts.output, null);
});

test('-o sets the output path', () => {
  const opts = parseArgs(['-o', 'out.md', 'chat.md']);
  assert.equal(opts.output, 'out.md');
  assert.equal(opts.file, 'chat.md');
});

test('--output sets the output path', () => {
  const opts = parseArgs(['--output', 'results/reply.md', 'chat.md']);
  assert.equal(opts.output, 'results/reply.md');
});

test('-o works after the file (order does not matter)', () => {
  const opts = parseArgs(['chat.md', '-o', 'reply.md']);
  assert.equal(opts.output, 'reply.md');
  assert.equal(opts.file, 'chat.md');
});

test('-o combined with -s and -m', () => {
  const opts = parseArgs(['-s', '-m', 'gpt-4o', '-o', 'reply.md', 'chat.md']);
  assert.equal(opts.stream, true);
  assert.equal(opts.model, 'gpt-4o');
  assert.equal(opts.output, 'reply.md');
  assert.equal(opts.file, 'chat.md');
});

test('-o without a value throws', () => {
  assert.throws(() => parseArgs(['-o']), /requires an output path/);
});

test('--output without a value throws', () => {
  assert.throws(() => parseArgs(['--output']), /requires an output path/);
});

test('-o with a flag-shaped value throws', () => {
  assert.throws(
    () => parseArgs(['-o', '-s']),
    /requires an output path/
  );
});

test('-- makes a flag-shaped output path a positional file', () => {
  const opts = parseArgs(['--', '-o']);
  assert.equal(opts.file, '-o');
  assert.equal(opts.output, null);
});

test('thinking defaults to null', () => {
  const opts = parseArgs(['chat.md']);
  assert.equal(opts.thinking, null);
});

test('-t yes enables thinking', () => {
  const opts = parseArgs(['-t', 'yes', 'chat.md']);
  assert.equal(opts.thinking, true);
  assert.equal(opts.file, 'chat.md');
});

test('-t no disables thinking', () => {
  const opts = parseArgs(['-t', 'no', 'chat.md']);
  assert.equal(opts.thinking, false);
});

test('--thinking=yes enables thinking', () => {
  const opts = parseArgs(['--thinking=yes', 'chat.md']);
  assert.equal(opts.thinking, true);
});

test('--thinking=no disables thinking', () => {
  const opts = parseArgs(['--thinking=no', 'chat.md']);
  assert.equal(opts.thinking, false);
});

test('--thinking with space-separated value', () => {
  const opts = parseArgs(['--thinking', 'yes', 'chat.md']);
  assert.equal(opts.thinking, true);
});

test('-t accepts true/false/1/0/on/off (case insensitive)', () => {
  for (const [val, expected] of [
    ['YES', true],
    ['true', true],
    ['1', true],
    ['on', true],
    ['NO', false],
    ['false', false],
    ['0', false],
    ['off', false],
  ]) {
    assert.equal(parseArgs(['-t', val]).thinking, expected, val);
  }
});

test('-t without a value throws', () => {
  assert.throws(() => parseArgs(['-t']), /requires a value/);
});

test('--thinking without a value throws', () => {
  assert.throws(() => parseArgs(['--thinking']), /requires a value/);
});

test('--thinking= with empty value throws', () => {
  assert.throws(() => parseArgs(['--thinking=']), /requires a value/);
});

test('--thinking with a flag-shaped value throws', () => {
  assert.throws(() => parseArgs(['-t', '-s']), /requires a value/);
});

test('-t with invalid value throws', () => {
  assert.throws(() => parseArgs(['-t', 'maybe']), /expects yes or no/);
});

test('--thinking=invalid throws', () => {
  assert.throws(() => parseArgs(['--thinking=maybe']), /expects yes or no/);
});

test('-t can appear after the file (order does not matter)', () => {
  const opts = parseArgs(['chat.md', '-t', 'yes']);
  assert.equal(opts.thinking, true);
  assert.equal(opts.file, 'chat.md');
});

test('parseThinkingValue: accepts truthy strings', () => {
  for (const v of ['yes', 'Yes', 'YES', 'true', '1', 'on', 'On']) {
    assert.equal(parseThinkingValue(v, '-t/--thinking'), true, v);
  }
});

test('parseThinkingValue: accepts falsy strings', () => {
  for (const v of ['no', 'No', 'NO', 'false', '0', 'off']) {
    assert.equal(parseThinkingValue(v, '-t/--thinking'), false, v);
  }
});

test('parseThinkingValue: throws on missing value', () => {
  assert.throws(() => parseThinkingValue(undefined, '-t/--thinking'), /requires a value/);
  assert.throws(() => parseThinkingValue(null, '-t/--thinking'), /requires a value/);
});

test('parseThinkingValue: throws on unknown value', () => {
  assert.throws(() => parseThinkingValue('maybe', '-t/--thinking'), /expects yes or no/);
  assert.throws(() => parseThinkingValue('', '-t/--thinking'), /expects yes or no/);
});

const {
  resolveConfig,
  buildModel,
  readEnvNumber,
  KNOWN_HEADER_FIELDS,
} = require('../src/cli.js');

const ENV_VARS = [
  'LANGCHAT_MODEL',
  'LANGCHAT_BASE_URL',
  'LANGCHAT_API_KEY',
  'OPENAI_API_KEY',
  ...KNOWN_HEADER_FIELDS.map((f) => f.env),
];

function withCleanEnv(fn) {
  const saved = {};
  for (const k of ENV_VARS) {
    saved[k] = process.env[k];
    delete process.env[k];
  }
  try {
    return fn();
  } finally {
    for (const k of ENV_VARS) {
      if (saved[k] === undefined) delete process.env[k];
      else process.env[k] = saved[k];
    }
  }
}

test('resolveConfig: model precedence CLI > header > env', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'env-model';
    assert.equal(
      resolveConfig({ cliModel: 'cli-model', header: { model: 'hdr-model' } }).model,
      'cli-model'
    );
    assert.equal(
      resolveConfig({ header: { model: 'hdr-model' } }).model,
      'hdr-model'
    );
    assert.equal(resolveConfig({}).model, 'env-model');
  });
});

test('resolveConfig: throws when no model is configured anywhere', () => {
  withCleanEnv(() => {
    assert.throws(() => resolveConfig({}), /LANGCHAT_MODEL is required/);
    assert.throws(
      () => resolveConfig({ header: {} }),
      /LANGCHAT_MODEL is required/
    );
  });
});

test('resolveConfig: streaming comes from CLI > header', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    assert.equal(
      resolveConfig({ cliModel: 'm', cliStream: true, header: { streaming: false } }).stream,
      true
    );
    assert.equal(
      resolveConfig({ cliModel: 'm', header: { streaming: true } }).stream,
      true
    );
    assert.equal(
      resolveConfig({ cliModel: 'm', header: { streaming: false } }).stream,
      false
    );
    assert.equal(resolveConfig({ cliModel: 'm' }).stream, false);
  });
});

test('resolveConfig: header fields override env defaults', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    process.env.LANGCHAT_TEMPERATURE = '0.9';
    process.env.LANGCHAT_MAX_TOKENS = '256';
    const cfg = resolveConfig({
      cliModel: 'm',
      header: { temperature: 0.2 },
    });
    assert.equal(cfg.temperature, 0.2);
    assert.equal(cfg.maxTokens, 256);
  });
});

test('resolveConfig: env vars populate known fields when header is silent', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    process.env.LANGCHAT_TEMPERATURE = '0.3';
    process.env.LANGCHAT_TIMEOUT = '5000';
    process.env.LANGCHAT_MAX_RETRIES = '7';
    const cfg = resolveConfig({ cliModel: 'm' });
    assert.equal(cfg.temperature, 0.3);
    assert.equal(cfg.timeout, 5000);
    assert.equal(cfg.maxRetries, 7);
  });
});

test('resolveConfig: unknown header keys (e.g. seed, thinking) stay in their original form', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    const cfg = resolveConfig({
      cliModel: 'm',
      header: { seed: 42, thinking: true, reasoning_effort: 'low' },
    });
    assert.equal(cfg.seed, 42);
    assert.equal(cfg.thinking, true);
    assert.equal(cfg.reasoning_effort, 'low');
    assert.equal(cfg.maxSeeds, undefined);
  });
});

test('resolveConfig: header snake_case keys are normalized to camelCase', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    const cfg = resolveConfig({
      cliModel: 'm',
      header: { max_tokens: 100, top_p: 0.8, max_retries: 4 },
    });
    assert.equal(cfg.maxTokens, 100);
    assert.equal(cfg.topP, 0.8);
    assert.equal(cfg.maxRetries, 4);
    assert.equal(cfg.max_tokens, undefined);
    assert.equal(cfg.top_p, undefined);
    assert.equal(cfg.max_retries, undefined);
  });
});

test('resolveConfig: passthrough fields land as-is on the config', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    const cfg = resolveConfig({
      cliModel: 'm',
      header: {
        thinking: true,
        reasoning_effort: 'high',
        stop: ['###'],
      },
    });
    assert.equal(cfg.thinking, true);
    assert.equal(cfg.reasoning_effort, 'high');
    assert.deepEqual(cfg.stop, ['###']);
  });
});

test('resolveConfig: thinking CLI true beats header false', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    const cfg = resolveConfig({
      cliModel: 'm',
      cliThinking: true,
      header: { thinking: false },
    });
    assert.equal(cfg.thinking, true);
  });
});

test('resolveConfig: thinking CLI false beats header true', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    const cfg = resolveConfig({
      cliModel: 'm',
      cliThinking: false,
      header: { thinking: true },
    });
    assert.equal(cfg.thinking, false);
  });
});

test('resolveConfig: no CLI thinking + header true sets thinking: true', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    const cfg = resolveConfig({
      cliModel: 'm',
      header: { thinking: true },
    });
    assert.equal(cfg.thinking, true);
  });
});

test('resolveConfig: no CLI thinking + header false sets thinking: false', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    const cfg = resolveConfig({
      cliModel: 'm',
      header: { thinking: false },
    });
    assert.equal(cfg.thinking, false);
  });
});

test('resolveConfig: no CLI thinking + no header leaves thinking unset', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    const cfg = resolveConfig({ cliModel: 'm' });
    assert.equal(cfg.thinking, undefined);
  });
});

test('resolveConfig: rejects an env var that is not a number', () => {
  withCleanEnv(() => {
    process.env.LANGCHAT_MODEL = 'm';
    process.env.LANGCHAT_TEMPERATURE = 'not-a-number';
    assert.throws(
      () => resolveConfig({ cliModel: 'm' }),
      /LANGCHAT_TEMPERATURE.*is not a valid number/
    );
  });
});

test('readEnvNumber returns undefined for missing or empty', () => {
  withCleanEnv(() => {
    assert.equal(readEnvNumber('LANGCHAT_TEMPERATURE'), undefined);
    process.env.LANGCHAT_TEMPERATURE = '';
    assert.equal(readEnvNumber('LANGCHAT_TEMPERATURE'), undefined);
    process.env.LANGCHAT_TEMPERATURE = '0.5';
    assert.equal(readEnvNumber('LANGCHAT_TEMPERATURE'), 0.5);
  });
});

test('buildModel: maps known fields to ChatOpenAI constructor params', () => {
  const m = buildModel({
    model: 'foo',
    apiKey: 'k',
    baseURL: 'http://x',
    stream: true,
    temperature: 0.4,
    maxTokens: 512,
    topP: 0.9,
    timeout: 1000,
    maxRetries: 3,
  });
  assert.equal(m.model, 'foo');
  assert.equal(m.streaming, true);
  assert.equal(m.temperature, 0.4);
  assert.equal(m.maxTokens, 512);
  assert.equal(m.topP, 0.9);
  assert.equal(m.timeout, 1000);
  assert.equal(m.lc_kwargs.maxRetries, 3);
  assert.equal(m.clientConfig.baseURL, 'http://x');
  assert.equal(m.useResponsesApi, false);
  assert.deepEqual(m.modelKwargs, {});
});

test('buildModel: unknown fields go to modelKwargs', () => {
  const m = buildModel({
    model: 'foo',
    thinking: true,
    reasoning_effort: 'medium',
    custom_thing: 'whatever',
  });
  assert.equal(m.model, 'foo');
  assert.deepEqual(m.modelKwargs, {
    thinking: true,
    reasoning_effort: 'medium',
    custom_thing: 'whatever',
  });
});

test('buildModel: attaches original config for structured-output rebuild', () => {
  const cfg = {
    model: 'foo',
    temperature: 0.2,
    thinking: true,
  };
  const m = buildModel(cfg);
  assert.equal(m._langchatConfig, cfg);
});

function tmpdir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'langchat-test-'));
}

test('resolveOutputPath: returns null when neither flag nor header set', () => {
  assert.equal(resolveOutputPath({}), null);
  assert.equal(resolveOutputPath({ header: {} }), null);
  assert.equal(resolveOutputPath({ header: { output: '' } }), null);
});

test('resolveOutputPath: CLI flag wins over header', () => {
  const resolved = resolveOutputPath({
    cliOutput: 'cli.md',
    header: { output: 'hdr.md' },
  });
  assert.equal(resolved, path.resolve('cli.md'));
});

test('resolveOutputPath: falls back to header when CLI is absent', () => {
  const resolved = resolveOutputPath({ header: { output: 'hdr.md' } });
  assert.equal(resolved, path.resolve('hdr.md'));
});

test('resolveOutputPath: resolves relative paths against cwd', () => {
  const resolved = resolveOutputPath({ cliOutput: 'out/reply.md' });
  assert.equal(resolved, path.resolve('out/reply.md'));
});

test('resolveOutputPath: rejects non-string header value', () => {
  assert.throws(
    () => resolveOutputPath({ header: { output: 42 } }),
    /header "output" must be a string path/
  );
});

test('writeResponse: writes to file and mirrors to stdout', () => {
  const dir = tmpdir();
  const file = path.join(dir, 'out.md');
  const restore = captureStdout();
  try {
    writeResponse('hello world\n', file);
    assert.equal(restore.captured(), 'hello world\n');
    assert.equal(fs.readFileSync(file, 'utf8'), 'hello world\n');
  } finally {
    restore.restore();
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('writeResponse: with null outputPath only writes to stdout', () => {
  const restore = captureStdout();
  try {
    writeResponse('just stdout\n', null);
    assert.equal(restore.captured(), 'just stdout\n');
  } finally {
    restore.restore();
  }
});

test('writeResponse: creates missing parent directories', () => {
  const dir = tmpdir();
  const file = path.join(dir, 'nested', 'deep', 'out.md');
  const restore = captureStdout();
  try {
    writeResponse('nested\n', file);
    assert.equal(fs.readFileSync(file, 'utf8'), 'nested\n');
  } finally {
    restore.restore();
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('writeResponse: overwrites existing file', () => {
  const dir = tmpdir();
  const file = path.join(dir, 'out.md');
  fs.writeFileSync(file, 'old content');
  const restore = captureStdout();
  try {
    writeResponse('new content\n', file);
    assert.equal(fs.readFileSync(file, 'utf8'), 'new content\n');
  } finally {
    restore.restore();
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('createLineWriter: without a fileStream, only writes to stdout', () => {
  const restore = captureStdout();
  try {
    const writer = createLineWriter();
    writer.write('line one\nline two');
    writer.write(' continued\n');
    writer.end();
    assert.equal(restore.captured(), 'line one\nline two continued\n');
  } finally {
    restore.restore();
  }
});

test('createLineWriter: with a fileStream, writes to both file and stdout', () => {
  const dir = tmpdir();
  const file = path.join(dir, 'streamed.md');
  const chunks = [];
  const fakeStream = {
    write(text) {
      chunks.push(text);
    },
  };
  const restore = captureStdout();
  try {
    const writer = createLineWriter(fakeStream);
    writer.write('first line\n');
    writer.write('partial ');
    writer.write('rest\n');
    writer.end();
    assert.equal(restore.captured(), 'first line\npartial rest\n');
    assert.deepEqual(chunks, ['first line\n', 'partial rest\n']);
    fs.writeFileSync(file, chunks.join(''));
    assert.equal(fs.readFileSync(file, 'utf8'), 'first line\npartial rest\n');
  } finally {
    restore.restore();
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('createLineWriter: with dim:true, wraps stdout output in ANSI dim but not file', () => {
  const chunks = [];
  const fakeStream = {
    write(text) {
      chunks.push(text);
    },
  };
  const restore = captureStdout();
  try {
    const writer = createLineWriter(fakeStream, { dim: true });
    writer.write('reasoning here\n');
    writer.end();
    const expected = '\x1b[2mreasoning here\n\x1b[0m';
    assert.equal(restore.captured(), expected);
    assert.deepEqual(chunks, ['reasoning here\n']);
  } finally {
    restore.restore();
  }
});

test('extractThinking: pulls reasoning_content from additional_kwargs (DeepSeek)', () => {
  const msg = {
    content: 'final answer',
    additional_kwargs: { reasoning_content: 'thinking...' },
  };
  assert.deepEqual(extractThinking(msg), {
    reasoning: 'thinking...',
    main: 'final answer',
  });
});

test('extractThinking: pulls thinking blocks (Anthropic)', () => {
  const msg = {
    content: [
      { type: 'thinking', text: 'let me think' },
      { type: 'text', text: 'final answer' },
    ],
  };
  assert.deepEqual(extractThinking(msg), {
    reasoning: 'let me think',
    main: 'final answer',
  });
});

test('extractThinking: pulls reasoning blocks (OpenAI)', () => {
  const msg = {
    content: [
      { type: 'reasoning', text: 'step 1' },
      { type: 'text', text: 'final answer' },
    ],
  };
  assert.deepEqual(extractThinking(msg), {
    reasoning: 'step 1',
    main: 'final answer',
  });
});

test('extractThinking: plain string content has no reasoning', () => {
  assert.deepEqual(extractThinking({ content: 'just text' }), {
    reasoning: '',
    main: 'just text',
  });
});

test('extractThinking: handles mixed blocks (multiple kinds, multiple instances)', () => {
  const msg = {
    additional_kwargs: { reasoning_content: 'A' },
    content: [
      { type: 'thinking', text: 'B' },
      { type: 'text', text: 'C' },
      { type: 'reasoning', text: 'D' },
      { type: 'text', text: 'E' },
    ],
  };
  assert.deepEqual(extractThinking(msg), {
    reasoning: 'ABD',
    main: 'CE',
  });
});

test('extractThinking: ignores non-object content blocks', () => {
  const msg = {
    content: [
      null,
      'raw string',
      42,
      { type: 'thinking', text: 'kept' },
      { type: 'text', text: 'answer' },
    ],
  };
  assert.deepEqual(extractThinking(msg), {
    reasoning: 'kept',
    main: 'answer',
  });
});

test('extractThinking: returns empty object on null input', () => {
  assert.deepEqual(extractThinking(null), { reasoning: '', main: '' });
  assert.deepEqual(extractThinking(undefined), { reasoning: '', main: '' });
});

test('extractThinking: reasoning block using ".reasoning" field instead of ".text"', () => {
  const msg = {
    content: [
      { type: 'reasoning', reasoning: 'via reasoning field' },
      { type: 'text', text: 'answer' },
    ],
  };
  assert.deepEqual(extractThinking(msg), {
    reasoning: 'via reasoning field',
    main: 'answer',
  });
});

function makeFakeModel(aiMessage) {
  return {
    async invoke() {
      return aiMessage;
    },
  };
}

test('runOnce: thinking=false suppresses reasoning on stdout', async () => {
  const model = makeFakeModel({
    text: 'final answer',
    additional_kwargs: { reasoning_content: 'hidden thinking' },
  });
  const restore = captureStdout();
  try {
    const reply = await runOnce(model, [], { thinking: false });
    assert.equal(restore.captured(), '');
    assert.equal(reply, 'final answer');
  } finally {
    restore.restore();
  }
});

test('runOnce: thinking=true prints reasoning dimmed then returns main reply', async () => {
  const model = makeFakeModel({
    text: 'final answer',
    additional_kwargs: { reasoning_content: 'reasoning text' },
  });
  const restore = captureStdout();
  try {
    const reply = await runOnce(model, [], { thinking: true });
    assert.equal(reply, 'final answer');
    assert.equal(
      restore.captured(),
      '\x1b[2mreasoning text\x1b[0m\n'
    );
  } finally {
    restore.restore();
  }
});

test('runOnce: thinking=true with no reasoning produces no stdout', async () => {
  const model = makeFakeModel({ text: 'final answer' });
  const restore = captureStdout();
  try {
    const reply = await runOnce(model, [], { thinking: true });
    assert.equal(reply, 'final answer');
    assert.equal(restore.captured(), '');
  } finally {
    restore.restore();
  }
});

test('runOnce: thinking=null is a no-op (existing behavior)', async () => {
  const model = makeFakeModel({
    text: 'final answer',
    additional_kwargs: { reasoning_content: 'secret' },
  });
  const restore = captureStdout();
  try {
    const reply = await runOnce(model, [], { thinking: null });
    assert.equal(reply, 'final answer');
    assert.equal(restore.captured(), '');
  } finally {
    restore.restore();
  }
});

function makeStreamingModel(chunks) {
  return {
    async stream() {
      return (async function* () {
        for (const c of chunks) yield c;
      })();
    },
  };
}

test('runStreamed: thinking=true prints reasoning dimmed, main to stdout+file', async () => {
  const chunks = [
    {
      content: [{ type: 'thinking', text: 'thinking ' }],
      additional_kwargs: { reasoning_content: 'A ' },
    },
    { content: 'answer part 1' },
    {
      content: [
        { type: 'thinking', text: 'more' },
        { type: 'text', text: ' part 2' },
      ],
    },
  ];
  const fileChunks = [];
  const fakeStream = {
    write(text) {
      fileChunks.push(text);
    },
  };
  const restore = captureStdout();
  try {
    const reply = await runStreamed(makeStreamingModel(chunks), [], {
      fileStream: fakeStream,
      thinking: true,
    });
    assert.equal(reply, undefined);
    const out = restore.captured();
    assert.match(out, /\x1b\[2mA thinking more\x1b\[0m/);
    assert.match(out, /answer part 1 part 2/);
    assert.equal(fileChunks.join(''), 'answer part 1 part 2');
    assert.ok(!fileChunks.join('').includes('\x1b['), 'file output has no ANSI');
  } finally {
    restore.restore();
  }
});

test('runStreamed: thinking=false drops reasoning from stdout', async () => {
  const chunks = [
    { content: 'main only' },
    {
      content: [{ type: 'thinking', text: 'should not appear' }],
      additional_kwargs: { reasoning_content: 'also hidden' },
    },
  ];
  const restore = captureStdout();
  try {
    await runStreamed(makeStreamingModel(chunks), [], { thinking: false });
    const out = restore.captured();
    assert.match(out, /main only/);
    assert.ok(!out.includes('should not appear'));
    assert.ok(!out.includes('also hidden'));
    assert.ok(!out.includes('\x1b['));
  } finally {
    restore.restore();
  }
});

test('runStreamed: thinking=null keeps current behavior (only main text)', async () => {
  const chunks = [
    { content: 'main' },
    { content: [{ type: 'thinking', text: 'IGNORED' }] },
  ];
  const restore = captureStdout();
  try {
    await runStreamed(makeStreamingModel(chunks), [], { thinking: null });
    const out = restore.captured();
    assert.match(out, /main/);
    assert.ok(!out.includes('IGNORED'));
  } finally {
    restore.restore();
  }
});

function captureStdout() {
  const chunks = [];
  const origWrite = process.stdout.write.bind(process.stdout);
  process.stdout.write = (chunk, ...rest) => {
    chunks.push(typeof chunk === 'string' ? chunk : chunk.toString());
    return true;
  };
  return {
    captured() {
      return chunks.join('');
    },
    restore() {
      process.stdout.write = origWrite;
    },
  };
}