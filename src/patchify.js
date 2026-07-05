'use strict';

const sharp = require('sharp');

const PATCHIFY_DIRECTIVE_RE =
  /\{\{\s*patchify\s+"([^"]+)"\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*\}\}/g;

const DEFAULT_TILE_MIME_TYPE = 'image/png';

function parsePatchifyArgs(match) {
  const m = PATCHIFY_DIRECTIVE_RE.exec(match);
  if (!m) {
    throw new Error(`patchify: cannot parse arguments from "${match}"`);
  }
  return {
    path: m[1],
    m: parseInt(m[2], 10),
    n: parseInt(m[3], 10),
    x: parseFloat(m[4]),
    y: parseFloat(m[5]),
  };
}

function validatePatchifyArgs(args) {
  const { m, n, x, y } = args;
  if (!Number.isInteger(m) || m < 1) {
    throw new Error(
      `patchify: m must be a positive integer, got ${m}`
    );
  }
  if (!Number.isInteger(n) || n < 1) {
    throw new Error(
      `patchify: n must be a positive integer, got ${n}`
    );
  }
  if (!Number.isFinite(x) || x < 0 || x >= 100) {
    throw new Error(
      `patchify: x (vertical overlap %) must be in [0, 100), got ${x}`
    );
  }
  if (!Number.isFinite(y) || y < 0 || y >= 100) {
    throw new Error(
      `patchify: y (horizontal overlap %) must be in [0, 100), got ${y}`
    );
  }
}

function computeGrid(W, H, m, n, xPct, yPct) {
  let tileH;
  let strideH;
  if (m === 1) {
    tileH = H;
    strideH = H;
  } else {
    tileH = H / (1 + (m - 1) * (1 - xPct / 100));
    strideH = tileH * (1 - xPct / 100);
  }

  let tileW;
  let strideW;
  if (n === 1) {
    tileW = W;
    strideW = W;
  } else {
    tileW = W / (1 + (n - 1) * (1 - yPct / 100));
    strideW = tileW * (1 - yPct / 100);
  }

  const tiles = [];
  for (let r = 0; r < m; r++) {
    for (let c = 0; c < n; c++) {
      let top = Math.round(r * strideH);
      let left = Math.round(c * strideW);
      if (r === m - 1 && m > 1) {
        top = Math.max(0, Math.round(H - tileH));
      }
      if (c === n - 1 && n > 1) {
        left = Math.max(0, Math.round(W - tileW));
      }
      tiles.push({
        row: r,
        col: c,
        left,
        top,
        width: Math.round(tileW),
        height: Math.round(tileH),
      });
    }
  }
  return tiles;
}

async function patchifyImage(buffer, { m, n, x, y } = {}) {
  validatePatchifyArgs({ m, n, x, y });

  const img = sharp(buffer);
  const meta = await img.metadata();
  const W = meta.width;
  const H = meta.height;
  if (!W || !H) {
    throw new Error(
      `patchify: cannot read image dimensions (got ${W}x${H})`
    );
  }

  const grid = computeGrid(W, H, m, n, x, y);
  const patches = [];
  for (const t of grid) {
    const extractLeft = Math.max(0, Math.min(W - 1, t.left));
    const extractTop = Math.max(0, Math.min(H - 1, t.top));
    const extractWidth = Math.max(1, Math.min(W - extractLeft, t.width));
    const extractHeight = Math.max(1, Math.min(H - extractTop, t.height));

    const patchBuf = await sharp(buffer)
      .extract({
        left: extractLeft,
        top: extractTop,
        width: extractWidth,
        height: extractHeight,
      })
      .png()
      .toBuffer();

    patches.push({
      row: t.row,
      col: t.col,
      left: extractLeft,
      top: extractTop,
      width: extractWidth,
      height: extractHeight,
      patch: patchBuf,
      mimeType: DEFAULT_TILE_MIME_TYPE,
    });
  }
  return patches;
}

function sourceLabelFor(rawPath, row, col) {
  const dot = rawPath.lastIndexOf('.');
  const base = dot > 0 ? rawPath.slice(0, dot) : rawPath;
  return `${base}[r${row}c${col}].png`;
}

module.exports = {
  PATCHIFY_DIRECTIVE_RE,
  parsePatchifyArgs,
  validatePatchifyArgs,
  computeGrid,
  patchifyImage,
  sourceLabelFor,
  DEFAULT_TILE_MIME_TYPE,
};
