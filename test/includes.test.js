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

async function expand(text, opts) {
  return (await resolveIncludes(text, opts)).text;
}

async function run(text, opts) {
  return await resolveIncludes(text, opts);
}

test('expands a simple include next to the chat file', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'chat.md', 'before {{ include "snippet.txt" }} after');
  writeFile(dir, 'snippet.txt', 'HELLO');
  const out = await expand(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
    baseDir: dir,
  });
  assert.equal(out, 'before HELLO after');
});

test('expands nested includes recursively', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'chat.md', '{{ include "level1.md" }}');
  writeFile(dir, 'level1.md', 'L1 {{ include "leaf.txt" }} L1');
  writeFile(dir, 'leaf.txt', 'LEAF');
  const out = await expand(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
    baseDir: dir,
  });
  assert.equal(out, 'L1 LEAF L1');
});

test('allows siblings to share an include (diamond shape)', async () => {
  const dir = makeTmpDir();
  writeFile(
    dir,
    'chat.md',
    '{{ include "a.md" }} || {{ include "b.md" }}'
  );
  writeFile(dir, 'a.md', '[A:{{ include "shared.txt" }}]');
  writeFile(dir, 'b.md', '[B:{{ include "shared.txt" }}]');
  writeFile(dir, 'shared.txt', 'S');
  const out = await expand(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
    baseDir: dir,
  });
  assert.equal(out, '[A:S] || [B:S]');
});

test('detects cyclic includes', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'a.md', '{{ include "b.md" }}');
  writeFile(dir, 'b.md', '{{ include "a.md" }}');
  await assert.rejects(
    () => run('{{ include "a.md" }}', { baseDir: dir }),
    /cyclic include detected/
  );
});

test('errors with a useful message when an included file is missing', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'chat.md', '{{ include "nope.txt" }}');
  await assert.rejects(
    () =>
      run(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
        baseDir: dir,
      }),
    /include failed: nope\.txt/
  );
});

test('blocks path traversal by default', async () => {
  const dir = makeTmpDir();
  const outside = makeTmpDir();
  writeFile(outside, 'secret.txt', 'SECRET');
  writeFile(dir, 'chat.md', '{{ include "../secret.txt" }}');
  await assert.rejects(
    () =>
      run(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
        baseDir: dir,
      }),
    /escapes base directory/
  );
});

test('permits escape when allowEscape is true', async () => {
  const parent = makeTmpDir();
  const dir = path.join(parent, 'inner');
  const outside = path.join(parent, 'outside');
  fs.mkdirSync(dir);
  fs.mkdirSync(outside);
  writeFile(outside, 'secret.txt', 'SECRET');
  writeFile(dir, 'chat.md', '{{ include "../outside/secret.txt" }}');
  const out = await expand(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
    baseDir: dir,
    allowEscape: true,
  });
  assert.equal(out, 'SECRET');
});

test('resolves absolute paths against the filesystem root', async () => {
  const dir = makeTmpDir();
  const target = path.join(dir, 'abs.txt');
  fs.writeFileSync(target, 'ABS');
  writeFile(dir, 'chat.md', `{{ include "${target.replace(/\\/g, '\\\\')}" }}`);
  const out = await expand(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
    baseDir: dir,
  });
  assert.equal(out, 'ABS');
});

test('respects maxDepth', async () => {
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
  await assert.rejects(
    () => run(`{{ include "${prev}" }}`, { baseDir: dir, maxDepth: 3 }),
    /include depth exceeded 3/
  );
});

test('accepts loose whitespace inside the directive', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'a.txt', 'A');
  writeFile(dir, 'b.txt', 'B');
  writeFile(dir, 'c.txt', 'C');
  const md = [
    '{{include "a.txt"}}',
    '{{  include  "b.txt"  }}',
    '{{\ninclude "c.txt"\n}}',
  ].join('|');
  const out = await expand(md, { baseDir: dir });
  assert.equal(out, 'A|B|C');
});

test('leaves text without directives unchanged', async () => {
  const dir = makeTmpDir();
  const md = 'plain text {{ not include }} {{ include }} {{include "x"}}';
  writeFile(dir, 'x', 'X');
  const out = await expand(md, { baseDir: dir });
  assert.equal(out, 'plain text {{ not include }} {{ include }} X');
});

test('expansion integrates with the chat parser (directive inside a # !user block)', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'chat.md', `# !user\n\nAnswer based on: {{ include "ctx.txt" }}\n`);
  writeFile(dir, 'ctx.txt', 'the context');
  const { parseChatFile } = require('../src/parser.js');
  const expanded = await run(
    fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
    { baseDir: dir }
  );
  const { messages, outputSchema } = parseChatFile(expanded.text, expanded.attachments);
  assert.equal(outputSchema, null);
  assert.equal(messages.length, 1);
  assert.equal(messages[0].content, 'Answer based on: the context');
});

test('rejects non-string input', async () => {
  await assert.rejects(() => resolveIncludes(null), TypeError);
  await assert.rejects(() => resolveIncludes(42), TypeError);
});

test('image include leaves directive in text and records an attachment', async () => {
  const dir = makeTmpDir();
  const png = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
  writeFile(dir, 'pic.png', png);
  writeFile(dir, 'chat.md', 'see {{ include "pic.png" }} here');
  const result = await run(
    fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
    { baseDir: dir }
  );
  assert.equal(result.text, 'see {{ include "pic.png" }} here');
  assert.equal(result.attachments.length, 1);
  assert.equal(result.attachments[0].type, 'image');
  assert.equal(result.attachments[0].mimeType, 'image/png');
  assert.equal(result.attachments[0].source, 'pic.png');
  assert.equal(result.attachments[0].data, png.toString('base64'));
});

test('maps every supported extension to its mime type', async () => {
  const dir = makeTmpDir();
  const fixtures = [
    ['a.png', 'image/png'],
    ['b.jpg', 'image/jpeg'],
    ['c.jpeg', 'image/jpeg'],
    ['d.gif', 'image/gif'],
    ['e.webp', 'image/webp'],
  ];
  const md = fixtures.map(([n]) => `{{ include "${n}" }}`).join('|');
  for (const [n, _] of fixtures) writeFile(dir, n, Buffer.from([0]));
  const result = await run(md, { baseDir: dir });
  assert.equal(result.attachments.length, fixtures.length);
  for (let i = 0; i < fixtures.length; i++) {
    const [_name, expectedMime] = fixtures[i];
    assert.equal(result.attachments[i].mimeType, expectedMime);
  }
});

test('attachments appear in left-to-right directive order', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'a.png', Buffer.from([1]));
  writeFile(dir, 'b.txt', 'TXT');
  writeFile(dir, 'c.jpg', Buffer.from([2]));
  writeFile(dir, 'chat.md', '1:{{ include "a.png" }} 2:{{ include "b.txt" }} 3:{{ include "c.jpg" }}');
  const result = await run(
    fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
    { baseDir: dir }
  );
  assert.equal(result.text, '1:{{ include "a.png" }} 2:TXT 3:{{ include "c.jpg" }}');
  assert.equal(result.attachments.length, 2);
  assert.equal(result.attachments[0].mimeType, 'image/png');
  assert.equal(result.attachments[1].mimeType, 'image/jpeg');
});

test('unknown extensions are still treated as text', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'doc.bmp', 'NOT-AN-IMAGE-BUT-STILL-TEXT');
  writeFile(dir, 'chat.md', '{{ include "doc.bmp" }}');
  const result = await run(
    fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
    { baseDir: dir }
  );
  assert.equal(result.text, 'NOT-AN-IMAGE-BUT-STILL-TEXT');
  assert.equal(result.attachments.length, 0);
});

test('rejects files exceeding the size cap', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'big.png', Buffer.alloc(64));
  writeFile(dir, 'chat.md', '{{ include "big.png" }}');
  await assert.rejects(
    () =>
      run(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
        baseDir: dir,
        maxBytes: 16,
      }),
    /limit is 16 bytes/
  );
});

test('size cap also applies to text includes', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'big.txt', 'x'.repeat(1024));
  writeFile(dir, 'chat.md', '{{ include "big.txt" }}');
  await assert.rejects(
    () =>
      run(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
        baseDir: dir,
        maxBytes: 64,
      }),
    /exceeds 64 bytes/
  );
});

test('image include still respects cycle detection', async () => {
  const dir = makeTmpDir();
  writeFile(dir, 'pic.png', Buffer.from([0]));
  writeFile(dir, 'chat.md', '{{ include "chat.md" }}');
  await assert.rejects(
    () =>
      run(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
        baseDir: dir,
      }),
    /cyclic include detected/
  );
});

test('image include still respects path-traversal block', async () => {
  const parent = makeTmpDir();
  const dir = path.join(parent, 'inner');
  fs.mkdirSync(dir);
  fs.writeFileSync(path.join(parent, 'pic.png'), Buffer.from([0]));
  writeFile(dir, 'chat.md', '{{ include "../pic.png" }}');
  await assert.rejects(
    () =>
      run(fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'), {
        baseDir: dir,
      }),
    /escapes base directory/
  );
});

test('image include still respects maxDepth', async () => {
  const dir = makeTmpDir();
  let prev = 'leaf.png';
  fs.writeFileSync(path.join(dir, 'leaf.png'), Buffer.from([0]));
  for (let i = 0; i < 10; i++) {
    const next = `level${i}.md`;
    fs.writeFileSync(
      path.join(dir, next),
      `{{ include "${prev}" }}`
    );
    prev = next;
  }
  await assert.rejects(
    () => run(`{{ include "${prev}" }}`, { baseDir: dir, maxDepth: 3 }),
    /include depth exceeded 3/
  );
});

test('nested text file can itself include an image', async () => {
  const dir = makeTmpDir();
  fs.writeFileSync(path.join(dir, 'inner.png'), Buffer.from([7, 7, 7]));
  writeFile(dir, 'helper.md', 'see {{ include "inner.png" }} end');
  writeFile(dir, 'chat.md', 'before {{ include "helper.md" }} after');
  const result = await run(
    fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
    { baseDir: dir }
  );
  assert.equal(result.text, 'before see {{ include "inner.png" }} end after');
  assert.equal(result.attachments.length, 1);
  assert.equal(result.attachments[0].mimeType, 'image/png');
  assert.equal(result.attachments[0].source, 'inner.png');
});
