'use strict';

const FRONTMATTER_DELIMITER_RE = /^---\s*$/;
const FRONTMATTER_OPEN_RE = /^---\s*$/;

const QUOTED_DOUBLE_RE = /^"(.*)"\s*$/;
const QUOTED_SINGLE_RE = /^'(.*)'\s*$/;
const INT_RE = /^-?\d+$/;
const FLOAT_RE = /^-?\d*\.\d+([eE][-+]?\d+)?$|^-?\d+[eE][-+]?\d+$/;
const KEY_RE = /^[a-zA-Z_][\w.-]*$/;

function parseScalarValue(raw) {
  const trimmed = (raw || '').trim();

  if (trimmed === '' || trimmed === '|' || trimmed === '>') {
    return '';
  }

  const dq = trimmed.match(QUOTED_DOUBLE_RE);
  if (dq) return dq[1];

  const sq = trimmed.match(QUOTED_SINGLE_RE);
  if (sq) return sq[1];

  if (trimmed === 'true') return true;
  if (trimmed === 'false') return false;
  if (trimmed === 'null' || trimmed === '~') return null;

  if (INT_RE.test(trimmed)) return parseInt(trimmed, 10);
  if (FLOAT_RE.test(trimmed)) return parseFloat(trimmed);

  return trimmed;
}

function parseFrontmatter(text) {
  if (typeof text !== 'string') {
    throw new TypeError('parseFrontmatter expects a string');
  }

  const normalized = text.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
  const lines = normalized.split('\n');

  if (lines.length === 0 || !FRONTMATTER_OPEN_RE.test(lines[0] || '')) {
    return { body: text, opts: {}, headerLines: 0 };
  }

  let closeIdx = -1;
  for (let i = 1; i < lines.length; i++) {
    if (FRONTMATTER_DELIMITER_RE.test(lines[i])) {
      closeIdx = i;
      break;
    }
  }

  if (closeIdx === -1) {
    return { body: text, opts: {}, headerLines: 0 };
  }

  const opts = {};
  for (let i = 1; i < closeIdx; i++) {
    const rawLine = lines[i];
    const lineNumber = i + 1;
    const trimmed = rawLine.trim();

    if (trimmed === '' || trimmed.startsWith('#')) continue;

    if (/^\s/.test(rawLine)) {
      throw new Error(
        `frontmatter at line ${lineNumber}: keys must not be indented (got "${rawLine}")`
      );
    }

    const colonIdx = rawLine.indexOf(':');
    if (colonIdx === -1) {
      throw new Error(
        `frontmatter at line ${lineNumber}: expected "key: value" but got "${rawLine}"`
      );
    }

    const key = rawLine.slice(0, colonIdx).trim();
    if (!key) {
      throw new Error(
        `frontmatter at line ${lineNumber}: empty key before ":"`
      );
    }
    if (!KEY_RE.test(key)) {
      throw new Error(
        `frontmatter at line ${lineNumber}: invalid key "${key}"`
      );
    }

    opts[key] = parseScalarValue(rawLine.slice(colonIdx + 1));
  }

  const headerLines = closeIdx + 1;
  const body = lines.slice(headerLines).join('\n');

  return { body, opts, headerLines };
}

module.exports = { parseFrontmatter, parseScalarValue };