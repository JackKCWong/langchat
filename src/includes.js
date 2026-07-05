'use strict';

const fs = require('node:fs');
const path = require('node:path');

const DIRECTIVE_RE = /\{\{\s*include\s+"([^"]+)"\s*\}\}/g;

function resolveIncludes(text, opts = {}) {
  if (typeof text !== 'string') {
    throw new TypeError('resolveIncludes expects a string');
  }

  const baseDir = path.resolve(opts.baseDir || process.cwd());
  const maxDepth = opts.maxDepth ?? 8;
  const allowEscape = opts.allowEscape ?? false;
  const stack = [];

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

      let content;
      try {
        content = fs.readFileSync(resolved, 'utf8');
      } catch (err) {
        throw new Error(
          `include failed: ${rawPath} (${resolved}): ${err.message}`
        );
      }

      stack.push(resolved);
      try {
        return expand(content, path.dirname(resolved), depth + 1);
      } finally {
        stack.pop();
      }
    });
  };

  return expand(text, baseDir, 0);
}

module.exports = { resolveIncludes, DIRECTIVE_RE };