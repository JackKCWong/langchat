'use strict';

const fs = require('node:fs');
const path = require('node:path');

const {
  PATCHIFY_DIRECTIVE_RE,
  parsePatchifyArgs,
  validatePatchifyArgs,
  patchifyImage,
  sourceLabelFor,
} = require('./patchify.js');

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

const SYNTHETIC_INCLUDE_RE = /^\{\{\s*include\s+"([^"]+)"\s*\}\}$/;

function resolveIncludes(text, opts = {}) {
  if (typeof text !== 'string') {
    return Promise.reject(new TypeError('resolveIncludes expects a string'));
  }

  const baseDir = path.resolve(opts.baseDir || process.cwd());
  const maxDepth = opts.maxDepth ?? 8;
  const allowEscape = opts.allowEscape ?? false;
  const imageExtensions = opts.imageExtensions || DEFAULT_IMAGE_EXTENSIONS;
  const imageMimeTypes = opts.imageMimeTypes || DEFAULT_IMAGE_MIME_TYPES;
  const maxBytes = opts.maxBytes ?? DEFAULT_MAX_BYTES;
  const debug = opts.debug ?? false;
  const stack = [];
  const attachments = [];

  const recordImage = (rawPath, resolved, buf) => {
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

  const resolvePath = (rawPath, fileDir) => {
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
    return resolved;
  };

  async function expand(snippet, fileDir, depth) {
    if (depth > maxDepth) {
      throw new Error(`include depth exceeded ${maxDepth}`);
    }

    const out = [];
    let cursor = 0;

    const prevLastIndex = DIRECTIVE_RE.lastIndex;
    DIRECTIVE_RE.lastIndex = 0;
    try {
      let m;
      while ((m = DIRECTIVE_RE.exec(snippet)) !== null) {
        if (m.index > cursor) {
          out.push(snippet.slice(cursor, m.index));
        }

        const rawPath = m[1];
        const resolved = resolvePath(rawPath, fileDir);

        const ext = path.extname(resolved).toLowerCase();
        const isImage = imageExtensions.has(ext);

        stack.push(resolved);
        try {
          if (isImage) {
            let buf;
            try {
              buf = fs.readFileSync(resolved);
            } catch (err) {
              throw new Error(
                `include failed: ${rawPath} (${resolved}): ${err.message}`
              );
            }
            recordImage(rawPath, resolved, buf);
            out.push(m[0]);
          } else {
            const content = readText(rawPath, resolved);
            const nested = await expand(
              content,
              path.dirname(resolved),
              depth + 1
            );
            out.push(nested);
          }
        } finally {
          stack.pop();
        }

        cursor = m.index + m[0].length;
      }
    } finally {
      DIRECTIVE_RE.lastIndex = prevLastIndex;
    }

    if (cursor < snippet.length) {
      out.push(snippet.slice(cursor));
    }

    return await applyPatchify(out.join(''), fileDir);
  }

  async function applyPatchify(text, fileDir) {
    const matches = [];
    const prevLastIndex = PATCHIFY_DIRECTIVE_RE.lastIndex;
    PATCHIFY_DIRECTIVE_RE.lastIndex = 0;
    try {
      let m;
      while ((m = PATCHIFY_DIRECTIVE_RE.exec(text)) !== null) {
        matches.push({ offset: m.index, length: m[0].length, raw: m[0], groups: m });
        if (m.index === PATCHIFY_DIRECTIVE_RE.lastIndex) {
          PATCHIFY_DIRECTIVE_RE.lastIndex++;
        }
      }
    } finally {
      PATCHIFY_DIRECTIVE_RE.lastIndex = prevLastIndex;
    }

    let result = text;
    for (let i = matches.length - 1; i >= 0; i--) {
      const { offset, length, raw } = matches[i];
      const args = parsePatchifyArgs(raw);
      validatePatchifyArgs(args);

      const resolved = resolvePath(args.path, fileDir);

      let buf;
      try {
        buf = fs.readFileSync(resolved);
      } catch (err) {
        throw new Error(
          `patchify failed: ${args.path} (${resolved}): ${err.message}`
        );
      }
      if (buf.length > maxBytes) {
        throw new Error(
          `patchify "${args.path}" (${resolved}) is ${buf.length} bytes; ` +
            `limit is ${maxBytes} bytes`
        );
      }

      stack.push(resolved);
      let patches;
      try {
        patches = await patchifyImage(buf, {
          m: args.m,
          n: args.n,
          x: args.x,
          y: args.y,
        });
      } finally {
        stack.pop();
      }

      const placeholders = [];
      let writtenDebug = 0;
      for (const p of patches) {
        const label = sourceLabelFor(args.path, p.row, p.col);
        attachments.push({
          type: 'image',
          mimeType: p.mimeType,
          data: p.patch.toString('base64'),
          source: label,
        });
        placeholders.push(`{{ include "${label}" }}`);
        if (debug) {
          const outPath = path.join(path.dirname(resolved), label);
          try {
            fs.writeFileSync(outPath, p.patch);
            writtenDebug += 1;
          } catch (err) {
            process.stderr.write(
              `[langchat] --debug failed to write ${outPath}: ${err.message}\n`
            );
          }
        }
      }
      if (debug && writtenDebug > 0) {
        process.stderr.write(
          `[langchat] --debug wrote ${writtenDebug} patch${writtenDebug === 1 ? '' : 'es'} next to ${resolved}\n`
        );
      }

      result =
        result.slice(0, offset) +
        placeholders.join(' ') +
        result.slice(offset + length);
    }
    return result;
  }

  return expand(text, baseDir, 0).then((finalText) => ({
    text: finalText,
    attachments,
  }));
}

module.exports = {
  resolveIncludes,
  DIRECTIVE_RE,
  PATCHIFY_DIRECTIVE_RE,
  SYNTHETIC_INCLUDE_RE,
  DEFAULT_IMAGE_EXTENSIONS,
  DEFAULT_IMAGE_MIME_TYPES,
  DEFAULT_MAX_BYTES,
};
