'use strict';

const fs = require('node:fs');
const path = require('node:path');

const { ChatOpenAI } = require('@langchain/openai');

const { parseChatFile } = require('./parser.js');

const USAGE = `Usage: langchat [options] <chat.md>

Options:
  -s, --stream   Stream the response token-by-token to stdout
  -h, --help     Show this help and exit

Environment (auto-loaded from ./.env if present; existing env vars win):
  LANGCHAT_MODEL      Model name (required), e.g. gpt-4o-mini
  LANGCHAT_BASE_URL   OpenAI-compatible base URL (optional)
  LANGCHAT_API_KEY    API key (optional; falls back to OPENAI_API_KEY)
`;

function printUsage() {
  process.stdout.write(USAGE);
}

function loadDotenv() {
  try {
    const envPath = path.resolve(process.cwd(), '.env');
    if (fs.existsSync(envPath)) {
      process.loadEnvFile(envPath);
    }
  } catch {
    // Missing or unreadable .env is not fatal; user may use real env vars.
  }
}

function parseArgs(argv) {
  const opts = { stream: false, file: null, help: false };
  const positional = [];
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '-h' || a === '--help') {
      opts.help = true;
    } else if (a === '-s' || a === '--stream') {
      opts.stream = true;
    } else if (a === '--') {
      positional.push(...argv.slice(i + 1));
      break;
    } else if (a.startsWith('-')) {
      throw new Error(`Unknown option: ${a}`);
    } else {
      positional.push(a);
    }
  }
  if (positional.length > 1) {
    throw new Error(`Expected exactly one <chat.md> argument, got ${positional.length}.`);
  }
  opts.file = positional[0] || null;
  return opts;
}

function resolveConfig() {
  const model = process.env.LANGCHAT_MODEL;
  if (!model) {
    throw new Error(
      'LANGCHAT_MODEL is required. Set it to the model name ' +
        '(e.g. export LANGCHAT_MODEL=gpt-4o-mini).'
    );
  }
  const baseURL = process.env.LANGCHAT_BASE_URL || undefined;
  let apiKey = process.env.LANGCHAT_API_KEY || process.env.OPENAI_API_KEY;
  if (!apiKey) {
    process.stderr.write(
      '[langchat] warning: no API key found in LANGCHAT_API_KEY or OPENAI_API_KEY; ' +
        'using placeholder "sk-no-key-needed" so local servers (Ollama, LM Studio, vLLM) work.\n'
    );
    apiKey = 'sk-no-key-needed';
  }
  return { model, baseURL, apiKey };
}

function buildModel({ model, baseURL, apiKey, stream }) {
  const fields = { model };
  if (apiKey) fields.apiKey = apiKey;
  if (baseURL) {
    fields.configuration = { baseURL };
  }
  if (stream) fields.streaming = true;
  return new ChatOpenAI(fields);
}

function stringifyContent(content) {
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return '';
  let out = '';
  for (const block of content) {
    if (block && typeof block === 'object' && block.type === 'text') {
      out += block.text || '';
    }
  }
  return out;
}

async function runChat(model, messages, { stream }) {
  if (stream) {
    let buf = '';
    const streamIter = await model.stream(messages);
    for await (const chunk of streamIter) {
      buf += stringifyContent(chunk.content);
    }
    return buf;
  }
  const aiMessage = await model.invoke(messages);
  return typeof aiMessage.text === 'string' ? aiMessage.text : stringifyContent(aiMessage.content);
}

async function main(argv) {
  loadDotenv();

  let opts;
  try {
    opts = parseArgs(argv);
  } catch (err) {
    process.stderr.write(`langchat: ${err.message}\n\n`);
    printUsage();
    process.exit(2);
  }

  if (opts.help) {
    printUsage();
    return;
  }

  if (!opts.file) {
    printUsage();
    process.exit(2);
  }

  const absPath = path.resolve(opts.file);
  let text;
  try {
    text = fs.readFileSync(absPath, 'utf8');
  } catch (err) {
    process.stderr.write(`langchat: cannot read ${opts.file}: ${err.message}\n`);
    process.exit(1);
  }

  let messages;
  try {
    messages = parseChatFile(text);
  } catch (err) {
    process.stderr.write(`langchat: failed to parse ${opts.file}: ${err.message}\n`);
    process.exit(1);
  }

  let config;
  try {
    config = resolveConfig();
  } catch (err) {
    process.stderr.write(`langchat: ${err.message}\n`);
    process.exit(2);
  }

  const model = buildModel({ ...config, stream: opts.stream });

  try {
    const reply = await runChat(model, messages, { stream: opts.stream });
    process.stdout.write(reply.endsWith('\n') ? reply : reply + '\n');
  } catch (err) {
    process.stderr.write(`langchat: request failed: ${err.message}\n`);
    process.exit(1);
  }
}

module.exports = { main, parseArgs };