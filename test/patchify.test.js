'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const sharp = require('sharp');

const {
  PATCHIFY_DIRECTIVE_RE,
  parsePatchifyArgs,
  validatePatchifyArgs,
  computeGrid,
  patchifyImage,
  sourceLabelFor,
  DEFAULT_TILE_MIME_TYPE,
} = require('../src/patchify.js');
const { resolveIncludes } = require('../src/includes.js');
const { parseChatFile } = require('../src/parser.js');
const { parseFrontmatter } = require('../src/frontmatter.js');

function makeTmpDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'langchat-patchify-'));
}

async function makePng(width, height, channels = 3) {
  return await sharp({
    create: {
      width,
      height,
      channels,
      background: { r: 0xff, g: 0, b: 0 },
    },
  })
    .png()
    .toBuffer();
}

test('PATCHIFY_DIRECTIVE_RE matches the canonical form', () => {
  PATCHIFY_DIRECTIVE_RE.lastIndex = 0;
  const m = PATCHIFY_DIRECTIVE_RE.exec(
    '{{ patchify "Goku.png", 2, 2, 5, 10 }}'
  );
  assert.ok(m);
  assert.equal(m[1], 'Goku.png');
  assert.equal(m[2], '2');
  assert.equal(m[3], '2');
  assert.equal(m[4], '5');
  assert.equal(m[5], '10');
});

test('PATCHIFY_DIRECTIVE_RE tolerates extra whitespace', () => {
  PATCHIFY_DIRECTIVE_RE.lastIndex = 0;
  const variants = [
    '{{patchify "a.png",2,3,0.5,10}}',
    '{{  patchify  "b.png" , 1 , 4 , 99 , 0  }}',
    '{{\npatchify "c.png",\n3,\n3,\n25,\n25\n}}',
  ];
  for (const v of variants) {
    PATCHIFY_DIRECTIVE_RE.lastIndex = 0;
    assert.ok(PATCHIFY_DIRECTIVE_RE.exec(v), `should match: ${v}`);
  }
});

test('parsePatchifyArgs extracts integers from the directive text', () => {
  const out = parsePatchifyArgs('{{ patchify "a.png", 2, 3, 25, 0.5 }}');
  assert.deepEqual(out, {
    path: 'a.png',
    m: 2,
    n: 3,
    x: 25,
    y: 0.5,
  });
});

test('parsePatchifyArgs throws on malformed input', () => {
  assert.throws(
    () => parsePatchifyArgs('not a directive'),
    /cannot parse arguments/
  );
});

test('validatePatchifyArgs accepts the MVP5 example', () => {
  assert.doesNotThrow(() =>
    validatePatchifyArgs({ m: 2, n: 2, x: 5, y: 10 })
  );
});

test('validatePatchifyArgs rejects non-positive m and n', () => {
  assert.throws(() => validatePatchifyArgs({ m: 0, n: 2, x: 0, y: 0 }), /m/);
  assert.throws(() => validatePatchifyArgs({ m: -1, n: 2, x: 0, y: 0 }), /m/);
  assert.throws(() => validatePatchifyArgs({ m: 2, n: 0, x: 0, y: 0 }), /n/);
  assert.throws(
    () => validatePatchifyArgs({ m: 2.5, n: 2, x: 0, y: 0 }),
    /m must be a positive integer/
  );
});

test('validatePatchifyArgs rejects x and y outside [0, 100)', () => {
  assert.throws(
    () => validatePatchifyArgs({ m: 2, n: 2, x: -1, y: 0 }),
    /x \(vertical overlap/,
  );
  assert.throws(
    () => validatePatchifyArgs({ m: 2, n: 2, x: 0, y: -5 }),
    /y \(horizontal overlap/,
  );
  assert.throws(
    () => validatePatchifyArgs({ m: 2, n: 2, x: 100, y: 0 }),
    /x \(vertical overlap/,
  );
  assert.throws(
    () => validatePatchifyArgs({ m: 2, n: 2, x: 0, y: 100 }),
    /y \(horizontal overlap/,
  );
});

test('computeGrid produces m*n tiles in row-major order', () => {
  const grid = computeGrid(200, 100, 2, 3, 0, 0);
  assert.equal(grid.length, 6);
  for (let r = 0; r < 2; r++) {
    for (let c = 0; c < 3; c++) {
      const i = r * 3 + c;
      assert.equal(grid[i].row, r);
      assert.equal(grid[i].col, c);
    }
  }
});

test('computeGrid at 0% overlap produces a perfect tiling', () => {
  const grid = computeGrid(100, 80, 2, 2, 0, 0);
  assert.equal(grid.length, 4);
  for (const t of grid) {
    assert.equal(t.width, 50);
    assert.equal(t.height, 40);
  }
  assert.deepEqual(grid[0], { row: 0, col: 0, left: 0, top: 0, width: 50, height: 40 });
  assert.deepEqual(grid[1], { row: 0, col: 1, left: 50, top: 0, width: 50, height: 40 });
  assert.deepEqual(grid[2], { row: 1, col: 0, left: 0, top: 40, width: 50, height: 40 });
  assert.deepEqual(grid[3], { row: 1, col: 1, left: 50, top: 40, width: 50, height: 40 });
});

test('computeGrid with vertical overlap slides rows and bottom-aligns the last row', () => {
  const grid = computeGrid(100, 80, 2, 2, 50, 0);
  assert.equal(grid[0].top, 0);
  assert.equal(grid[1].top, 0);
  assert.equal(grid[2].top + grid[2].height, 80);
  assert.equal(grid[3].top + grid[3].height, 80);
});

test('computeGrid with horizontal overlap slides columns and right-aligns the last column', () => {
  const grid = computeGrid(100, 80, 2, 2, 0, 50);
  assert.equal(grid[0].left, 0);
  assert.equal(grid[2].left, 0);
  for (const r of [0, 1]) {
    assert.equal(grid[r * 2 + 1].left + grid[r * 2 + 1].width, 100);
  }
});

test('computeGrid with m=1 covers the full height regardless of x', () => {
  const grid = computeGrid(100, 80, 1, 4, 99, 0);
  assert.equal(grid.length, 4);
  for (const t of grid) {
    assert.equal(t.height, 80);
    assert.equal(t.top, 0);
  }
  assert.equal(grid[0].left, 0);
  assert.equal(grid[3].left + grid[3].width, 100);
});

test('computeGrid with n=1 covers the full width regardless of y', () => {
  const grid = computeGrid(100, 80, 4, 1, 0, 99);
  assert.equal(grid.length, 4);
  for (const t of grid) {
    assert.equal(t.width, 100);
    assert.equal(t.left, 0);
  }
});

test('sourceLabelFor produces "<base>[r<row>c<col>].png"', () => {
  assert.equal(sourceLabelFor('Goku.png', 0, 0), 'Goku[r0c0].png');
  assert.equal(sourceLabelFor('Goku.png', 1, 2), 'Goku[r1c2].png');
  assert.equal(sourceLabelFor('nested/path/a.bmp', 0, 0), 'a[r0c0].png');
  assert.equal(sourceLabelFor('noext', 2, 3), 'noext[r2c3].png');
});

test('patchifyImage with 1x1 produces one tile covering the whole image', async () => {
  const buf = await makePng(40, 30);
  const tiles = await patchifyImage(buf, { m: 1, n: 1, x: 5, y: 10 });
  assert.equal(tiles.length, 1);
  assert.equal(tiles[0].row, 0);
  assert.equal(tiles[0].col, 0);
  assert.equal(tiles[0].width, 40);
  assert.equal(tiles[0].height, 30);
  assert.equal(tiles[0].left, 0);
  assert.equal(tiles[0].top, 0);
  assert.equal(tiles[0].mimeType, DEFAULT_TILE_MIME_TYPE);
  const meta = await sharp(tiles[0].patch).metadata();
  assert.equal(meta.format, 'png');
  assert.equal(meta.width, 40);
  assert.equal(meta.height, 30);
});

test('patchifyImage produces m*n tiles re-encoded as PNG', async () => {
  const buf = await makePng(100, 80);
  const tiles = await patchifyImage(buf, { m: 2, n: 2, x: 5, y: 10 });
  assert.equal(tiles.length, 4);
  for (const t of tiles) {
    assert.equal(t.mimeType, 'image/png');
    const meta = await sharp(t.patch).metadata();
    assert.equal(meta.format, 'png');
    assert.ok(meta.width > 0);
    assert.ok(meta.height > 0);
  }
});

test('patchifyImage with 0% overlap partitions the image exactly', async () => {
  const buf = await makePng(100, 80);
  const tiles = await patchifyImage(buf, { m: 2, n: 2, x: 0, y: 0 });
  assert.equal(tiles.length, 4);
  for (const t of tiles) {
    assert.equal(t.width, 50);
    assert.equal(t.height, 40);
  }
  const byKey = (r, c) => tiles.find((t) => t.row === r && t.col === c);
  assert.equal(byKey(0, 0).left, 0);
  assert.equal(byKey(0, 0).top, 0);
  assert.equal(byKey(0, 1).left, 50);
  assert.equal(byKey(1, 0).top, 40);
  assert.equal(byKey(1, 1).left, 50);
  assert.equal(byKey(1, 1).top, 40);
});

test('patchifyImage re-encodes the source regardless of source mime', async () => {
  const png = await makePng(60, 40, 4);
  const tiles = await patchifyImage(png, { m: 2, n: 2, x: 0, y: 0 });
  for (const t of tiles) {
    assert.equal(t.mimeType, 'image/png');
    const meta = await sharp(t.patch).metadata();
    assert.equal(meta.format, 'png');
  }
});

test('resolveIncludes expands a patchify directive into placeholders + attachments', async () => {
  const dir = makeTmpDir();
  const imgPath = path.join(dir, 'a.png');
  fs.writeFileSync(imgPath, await makePng(100, 80));
  write(dir, 'chat.md', 'see {{ patchify "a.png", 2, 2, 0, 0 }} here');

  const result = await resolveIncludes(
    fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
    { baseDir: dir }
  );

  assert.equal(
    result.text,
    'see {{ include "a[r0c0].png" }} {{ include "a[r0c1].png" }} {{ include "a[r1c0].png" }} {{ include "a[r1c1].png" }} here'
  );
  assert.equal(result.attachments.length, 4);
  for (const a of result.attachments) {
    assert.equal(a.type, 'image');
    assert.equal(a.mimeType, 'image/png');
  }
  assert.deepEqual(
    result.attachments.map((a) => a.source),
    ['a[r0c0].png', 'a[r0c1].png', 'a[r1c0].png', 'a[r1c1].png']
  );
});

test('resolveIncludes with patchify preserves directive order alongside text and image includes', async () => {
  const dir = makeTmpDir();
  const bg = await makePng(100, 80);
  const standalone = Buffer.from([0xff, 0xd8, 0xff, 0xd8]);
  fs.writeFileSync(path.join(dir, 'a.png'), bg);
  fs.writeFileSync(path.join(dir, 'b.jpg'), standalone);
  write(
    dir,
    'chat.md',
    '1:{{ include "b.jpg" }} 2:{{ patchify "a.png", 2, 2, 0, 0 }} 3:end'
  );

  const result = await resolveIncludes(
    fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
    { baseDir: dir }
  );

  assert.equal(result.attachments.length, 5);
  assert.equal(result.attachments[0].source, 'b.jpg');
  assert.equal(result.attachments[1].source, 'a[r0c0].png');
  assert.equal(result.attachments[2].source, 'a[r0c1].png');
  assert.equal(result.attachments[3].source, 'a[r1c0].png');
  assert.equal(result.attachments[4].source, 'a[r1c1].png');

  assert.ok(result.text.includes('1:{{ include "b.jpg" }}'));
  assert.ok(result.text.includes('2:{{ include "a[r0c0].png" }}'));
  assert.ok(result.text.includes('3:end'));
});

test('resolveIncludes blocks path traversal on patchify', async () => {
  const dir = makeTmpDir();
  const outside = makeTmpDir();
  fs.writeFileSync(path.join(outside, 'secret.png'), await makePng(40, 40));
  write(dir, 'chat.md', '{{ patchify "../secret.png", 2, 2, 0, 0 }}');

  await assert.rejects(
    () =>
      resolveIncludes(
        fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
        { baseDir: dir }
      ),
    /escapes base directory/
  );
});

test('resolveIncludes rejects invalid patchify arguments', async () => {
  const dir = makeTmpDir();
  fs.writeFileSync(path.join(dir, 'a.png'), await makePng(100, 80));
  write(dir, 'chat.md', '{{ patchify "a.png", 0, 2, 0, 0 }}');

  await assert.rejects(
    () =>
      resolveIncludes(
        fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
        { baseDir: dir }
      ),
    /m must be a positive integer/
  );
});

test('resolveIncludes rejects x >= 100', async () => {
  const dir = makeTmpDir();
  fs.writeFileSync(path.join(dir, 'a.png'), await makePng(100, 80));
  write(dir, 'chat.md', '{{ patchify "a.png", 2, 2, 100, 0 }}');

  await assert.rejects(
    () =>
      resolveIncludes(
        fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
        { baseDir: dir }
      ),
    /x \(vertical overlap/
  );
});

test('resolveIncludes rejects missing patchify source', async () => {
  const dir = makeTmpDir();
  write(dir, 'chat.md', '{{ patchify "nope.png", 2, 2, 0, 0 }}');

  await assert.rejects(
    () =>
      resolveIncludes(
        fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
        { baseDir: dir }
      ),
    /patchify failed: nope\.png/
  );
});

test('resolveIncludes applies the size cap to patchify input', async () => {
  const dir = makeTmpDir();
  fs.writeFileSync(path.join(dir, 'big.png'), Buffer.alloc(64));
  write(dir, 'chat.md', '{{ patchify "big.png", 2, 2, 0, 0 }}');

  await assert.rejects(
    () =>
      resolveIncludes(
        fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
        { baseDir: dir, maxBytes: 16 }
      ),
    /limit is 16 bytes/
  );
});

test('parser.js consumes patchify placeholders as image attachments in order', async () => {
  const dir = makeTmpDir();
  const bg = await makePng(100, 80);
  fs.writeFileSync(path.join(dir, 'a.png'), bg);

  const md = [
    '---',
    'model: qwen-vl-plus',
    '---',
    '',
    '# !user',
    '',
    'See {{ patchify "a.png", 2, 2, 0, 0 }} please',
    '',
  ].join('\n');

  const { body } = parseFrontmatter(md);
  const expanded = await resolveIncludes(body, { baseDir: dir });
  const { messages, outputSchema } = parseChatFile(
    expanded.text,
    expanded.attachments
  );
  assert.equal(outputSchema, null);
  assert.equal(messages.length, 1);
  const blocks = messages[0].content;
  assert.ok(Array.isArray(blocks));
  const images = blocks.filter((b) => b.type === 'image');
  assert.equal(images.length, 4);
  assert.deepEqual(
    images.map((b) => b.source),
    ['a[r0c0].png', 'a[r0c1].png', 'a[r1c0].png', 'a[r1c1].png']
  );
});

test('resolveIncludes handles a text-included file that itself uses patchify', async () => {
  const dir = makeTmpDir();
  fs.writeFileSync(path.join(dir, 'a.png'), await makePng(60, 40));
  write(dir, 'helper.md', 'P={{ patchify "a.png", 1, 2, 0, 0 }}');
  write(dir, 'chat.md', 'before {{ include "helper.md" }} after');

  const result = await resolveIncludes(
    fs.readFileSync(path.join(dir, 'chat.md'), 'utf8'),
    { baseDir: dir }
  );

  assert.equal(
    result.text,
    'before P={{ include "a[r0c0].png" }} {{ include "a[r0c1].png" }} after'
  );
  assert.equal(result.attachments.length, 2);
});

function write(dir, name, contents) {
  fs.writeFileSync(path.join(dir, name), contents);
}
