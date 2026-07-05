'use strict';

const {
  SystemMessage,
  HumanMessage,
  AIMessage,
} = require('@langchain/core/messages');

const HEADER_RE = /^#!(system|user|assistant)\s*$/;

const ROLE_FACTORIES = {
  system: SystemMessage,
  user: HumanMessage,
  assistant: AIMessage,
};

function parseChatFile(text) {
  if (typeof text !== 'string') {
    throw new TypeError('parseChatFile expects a string');
  }

  const lines = text.split(/\r?\n/);
  const messages = [];
  let currentRole = null;
  let currentLines = [];
  let currentHeaderLine = -1;

  const flush = () => {
    if (currentRole === null) return;
    const content = currentLines.join('\n').replace(/^\n+|\n+$/g, '');
    const Factory = ROLE_FACTORIES[currentRole];
    messages.push(new Factory(content));
    currentRole = null;
    currentLines = [];
    currentHeaderLine = -1;
  };

  lines.forEach((line, idx) => {
    const lineNumber = idx + 1;
    const match = line.match(HEADER_RE);
    if (match) {
      flush();
      currentRole = match[1];
      currentHeaderLine = lineNumber;
      return;
    }
    if (line.startsWith('#!')) {
      const token = line.slice(2).trim().split(/\s+/)[0] || '';
      throw new Error(
        `Unknown role "#!${token}" at line ${lineNumber}. ` +
          `Expected one of: ${Object.keys(ROLE_FACTORIES).join(', ')}.`
      );
    }
    if (currentRole === null) {
      throw new Error(
        `Unexpected content at line ${lineNumber}: ` +
          `messages must begin with a "#!<role>" header.`
      );
    }
    currentLines.push(line);
  });

  flush();

  if (messages.length === 0) {
    throw new Error(
      'No messages found. The file must contain at least one "#!system", "#!user", or "#!assistant" block.'
    );
  }

  return messages;
}

module.exports = { parseChatFile };