'use strict';

const {
  SystemMessage,
  HumanMessage,
  AIMessage,
} = require('@langchain/core/messages');

const HEADER_RE = /^# !(system|user|assistant|output)\s*$/;
const DIRECTIVE_RE = /\{\{\s*include\s+"([^"]+)"\s*\}\}/g;

const ROLE_FACTORIES = {
  system: SystemMessage,
  user: HumanMessage,
  assistant: AIMessage,
};

function stripCodeFence(text) {
  const match = text.match(/^```[a-zA-Z]*\s*\n([\s\S]*?)\n```\s*$/);
  return match ? match[1] : text;
}

function parseOutputSchema(rawContent, lineNumber) {
  let text = (rawContent || '').trim();
  if (!text) {
    throw new Error(`# !output block at line ${lineNumber} is empty`);
  }
  text = stripCodeFence(text).trim();
  if (!text) {
    throw new Error(`# !output block at line ${lineNumber} is empty`);
  }
  try {
    return JSON.parse(text);
  } catch (err) {
    throw new Error(
      `# !output block at line ${lineNumber} is not valid JSON: ${err.message}`
    );
  }
}

function expandUserContent(rawText, attachments, role, lineNumber, startIdx) {
  if (!DIRECTIVE_RE.test(rawText)) {
    DIRECTIVE_RE.lastIndex = 0;
    return { content: rawText, nextIdx: startIdx };
  }
  DIRECTIVE_RE.lastIndex = 0;

  if (role !== 'user') {
    const m = rawText.match(DIRECTIVE_RE);
    const snippet = m ? m[0] : '';
    throw new Error(
      `image include ${snippet} at line ${lineNumber} is only supported ` +
        `inside a # !user block (found # !${role})`
    );
  }

  const blocks = [];
  let cursor = 0;
  let idx = startIdx;
  rawText.replace(DIRECTIVE_RE, (match, _rawPath, offset) => {
    if (offset > cursor) {
      blocks.push({ type: 'text', text: rawText.slice(cursor, offset) });
    }
    if (idx >= attachments.length) {
      throw new Error(
        `image include ${match} at line ${lineNumber} has no remaining ` +
          `attachment (directives exceed attachments)`
      );
    }
    blocks.push(attachments[idx]);
    idx += 1;
    cursor = offset + match.length;
    return match;
  });
  if (cursor < rawText.length) {
    blocks.push({ type: 'text', text: rawText.slice(cursor) });
  }

  return { content: blocks, nextIdx: idx };
}

function parseChatFile(text, attachments = [], opts = {}) {
  if (typeof text !== 'string') {
    throw new TypeError('parseChatFile expects a string');
  }
  if (!Array.isArray(attachments)) {
    throw new TypeError('parseChatFile attachments must be an array');
  }
  const lineOffset = Number.isFinite(opts.lineOffset) ? opts.lineOffset : 0;
  if (lineOffset < 0) {
    throw new RangeError('parseChatFile lineOffset must be >= 0');
  }

  const lines = text.split(/\r?\n/);
  const messages = [];
  let currentRole = null;
  let currentLines = [];
  let currentHeaderLine = -1;
  let attachmentIdx = 0;

  let outputRaw = null;
  let outputLine = -1;
  let outputSeen = false;

  const flush = () => {
    if (currentRole === null) return;
    const raw = currentLines.join('\n').replace(/^\n+|\n+$/g, '');
    if (currentRole === 'output') {
      if (outputSeen) {
        throw new Error(
          `duplicate # !output block at line ${currentHeaderLine}; ` +
            'only one # !output block is allowed per chat file.'
        );
      }
      outputSeen = true;
      outputRaw = raw;
      outputLine = currentHeaderLine;
      currentRole = null;
      currentLines = [];
      currentHeaderLine = -1;
      return;
    }
    const { content, nextIdx } = expandUserContent(
      raw,
      attachments,
      currentRole,
      currentHeaderLine,
      attachmentIdx
    );
    attachmentIdx = nextIdx;
    const Factory = ROLE_FACTORIES[currentRole];
    if (Array.isArray(content)) {
      messages.push(new Factory({ contentBlocks: content }));
    } else {
      messages.push(new Factory(content));
    }
    currentRole = null;
    currentLines = [];
    currentHeaderLine = -1;
  };

  lines.forEach((line, idx) => {
    const lineNumber = idx + 1 + lineOffset;
    const match = line.match(HEADER_RE);
    if (match) {
      flush();
      currentRole = match[1];
      currentHeaderLine = lineNumber;
      return;
    }
    if (line.startsWith('# !')) {
      const token = line.slice(3).trim().split(/\s+/)[0] || '';
      throw new Error(
        `Unknown role "# !${token}" at line ${lineNumber}. ` +
          `Expected one of: ${Object.keys(ROLE_FACTORIES).join(', ')}, output.`
      );
    }
    if (currentRole === null) {
      if (line.trim() === '') return;
      throw new Error(
        `Unexpected content at line ${lineNumber}: ` +
          `messages must begin with a "# !<role>" header.`
      );
    }
    currentLines.push(line);
  });

  flush();

  if (attachmentIdx !== attachments.length) {
    throw new Error(
      `attachment count mismatch: ${attachmentIdx} image directive(s) consumed ` +
        `but ${attachments.length} attachment(s) provided`
    );
  }

  if (messages.length === 0) {
    throw new Error(
      'No messages found. The file must contain at least one "# !system", "# !user", or "# !assistant" block.'
    );
  }

  const outputSchema = outputSeen
    ? parseOutputSchema(outputRaw, outputLine)
    : null;

  return { messages, outputSchema };
}

module.exports = { parseChatFile, parseOutputSchema };