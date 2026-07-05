'use strict';

const fs = require('node:fs');
const path = require('node:path');

const DEFAULT_IMAGE_EXTENSIONS = new Set([
  '.png',
  '.jpg',
  '.jpeg',
  '.gif',
  '.webp',
]);

const DEFAULT_IMAGE_MIME_TYPES = {
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.webp': 'image/webp',
};

const DEFAULT_MAX_BYTES = 5 * 1024 * 1024;

const DIRECTIVE_RE = /\{\{\s*include\s+"([^"]+)"\s*\}\}/g;

function resolveIncludes(text, opts = {}) {
  if (typeof text !== 'string') {
    throw new TypeError('resolveIncludes expects a string');
  }

  const baseDir = path.resolve(opts.baseDir || process.cwd());
  const maxDepth = opts.maxDepth ?? 8;
  const allowEscape = opts.allowEscape ?? false;
  const imageExtensions = opts.imageExtensions || DEFAULT_IMAGE_EXTENSIONS;
  const imageMimeTypes = opts.imageMimeTypes || DEFAULT_IMAGE_MIME_TYPES;
  const maxBytes = opts.maxBytes ?? DEFAULT_MAX_BYTES;

  const stack = [];
  const attachments = [];

  const recordImage = (rawPath, resolved) => {
    let buf;
    try {
      buf = fs.readFileSync(resolved);
    } catch (err) {
      throw new Error(
        `include failed: ${rawPath} (${resolved}): ${err.message}`
      );
    }
    if (buf.length > maxBytes) {
      throw new Error(
        `include "${rawPath}" (${resolved}) is ${buf.length} bytes; ` +
          `limit is ${maxBytes} bytes`
      );
    }
    const ext = path.extname(resolved).toLowerCase();
    const mimeType = imageMimeTypes[ext];
    attachments.push({
      type: 'image',
      mimeType,
      data: buf.toString('base64'),
      source: rawPath,
    });
  };

  const readText = (rawPath, resolved) => {
    let content;
    try {
      content = fs.readFileSync(resolved, 'utf8');
    } catch (err) {
      throw new Error(
        `include failed: ${rawPath} (${resolved}): ${err.message}`
      );
    }
    if (Buffer.byteLength(content, 'utf8') > maxBytes) {
      throw new Error(
        `include "${rawPath}" (${resolved}) exceeds ${maxBytes} bytes`
      );
    }
    return content;
  };

  const expand = (snippet, fileDir, depth) => {
    if (depth > maxDepth) {
      throw new Error(`include depth exceeded ${maxDepth}`);
    }
    return snippet.replace(DIRECTIVE_RE, (_match, rawPath) => {
      const resolved = path.isAbsolute(rawPath)
        ? path.resolve(rawPath)
        : path.resolve(fileDir, rawPath);

      const insideBase =
        resolved === baseDir || resolved.startsWith(baseDir + path.sep);
      if (!insideBase && !allowEscape) {
        throw new Error(
          `include "${rawPath}" escapes base directory ${baseDir}`
        );
      }

      if (stack.includes(resolved)) {
        throw new Error(
          `cyclic include detected: ${stack.join(' -> ')} -> ${resolved}`
        );
      }

      const ext = path.extname(resolved).toLowerCase();
      const isImage = imageExtensions.has(ext);

      stack.push(resolved);
      try {
        if (isImage) {
          recordImage(rawPath, resolved);
          return _match;
        }
        const content = readText(rawPath, resolved);
        return expand(content, path.dirname(resolved), depth + 1);
      } finally {
        stack.pop();
      }
    });
  };

  const newText = expand(text, baseDir, 0);
  return { text: newText, attachments };
}

module.exports = {
  resolveIncludes,
  DIRECTIVE_RE,
  DEFAULT_IMAGE_EXTENSIONS,
  DEFAULT_IMAGE_MIME_TYPES,
  DEFAULT_MAX_BYTES,
};