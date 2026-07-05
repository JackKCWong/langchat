'use strict';

const fs = require('node:fs');
const path = require('node:path');

const { ChatOpenAI } = require('@langchain/openai');

const { parseChatFile } = require('./parser.js');
const { resolveIncludes } = require('./includes.js');

const USAGE = `Usage: langchat [options] <chat.md>

Options:
  -m, --model <name>         Model name (overrides LANGCHAT_MODEL)
  -s, --stream               Stream the response token-by-token to stdout
      --allow-include-escape  Permit {{ include }} paths outside the chat file's directory
  -h, --help                 Show this help and exit

Use a "# !output" block containing a JSON Schema to constrain the response shape.
When present, the model returns a parsed object which is pretty-printed as JSON.

Environment (auto-loaded from ./.env if present; existing env vars win):
  LANGCHAT_MODEL      Model name (required if -m/--model not given), e.g. gpt-4o-mini
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
  const opts = {
    stream: false,
    file: null,
    help: false,
    allowIncludeEscape: false,
    model: null,
  };
  const positional = [];
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '-h' || a === '--help') {
      opts.help = true;
    } else if (a === '-s' || a === '--stream') {
      opts.stream = true;
    } else if (a === '--allow-include-escape') {
      opts.allowIncludeEscape = true;
    } else if (a === '-m' || a === '--model') {
      const value = argv[++i];
      if (value === undefined || value.startsWith('-')) {
        throw new Error(`Option ${a} requires a model name.`);
      }
      opts.model = value;
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

function resolveConfig(overrides = {}) {
  const model = overrides.model || process.env.LANGCHAT_MODEL;
  if (!model) {
    throw new Error(
      'LANGCHAT_MODEL is required. Set it to the model name ' +
        '(e.g. export LANGCHAT_MODEL=gpt-4o-mini) or pass -m/--model.'
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
  if (RESPONSES_API_PREFERRED_MODEL_RE.test(model)) {
    process.stderr.write(
      `[langchat] warning: model "${model}" routes to the /v1/responses ` +
        'endpoint in @langchain/openai, which uses a different streaming protocol. ' +
        'For OpenAI-compatible servers (Ollama, LM Studio, vLLM, DeepSeek), use a ' +
        'chat-completions model name like "gpt-4o-mini", "llama3.1", or "deepseek-chat".\n'
    );
  }
  return { model, baseURL, apiKey };
}

const RESPONSES_API_PREFERRED_MODEL_RE = /gpt-5\.[2-9]-pro|gpt-5\.[2-9]\.[2-9]-pro|codex/;

function buildModel({ model, baseURL, apiKey, stream }) {
  const fields = { model };
  if (apiKey) fields.apiKey = apiKey;
  if (baseURL) {
    fields.configuration = { baseURL };
  }
  if (stream) fields.streaming = true;
  // Force the Chat Completions endpoint (/v1/chat/completions) regardless
  // of model name. Without this, @langchain/openai >=1.x can route to the
  // /v1/responses endpoint for certain models (e.g. gpt-5.2-pro, codex),
  // which has a different streaming protocol and breaks `-s` for users
  // hitting an OpenAI-compatible /chat/completions server.
  fields.useResponsesApi = false;
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

async function runOnce(model, messages) {
  const aiMessage = await model.invoke(messages);
  return typeof aiMessage.text === 'string'
    ? aiMessage.text
    : stringifyContent(aiMessage.content);
}

function createLineWriter() {
  let buffer = '';
  return {
    write(text) {
      if (!text) return;
      buffer += text;
      let nl;
      while ((nl = buffer.indexOf('\n')) !== -1) {
        const line = buffer.slice(0, nl);
        buffer = buffer.slice(nl + 1);
        process.stdout.write(line + '\n');
      }
    },
    end() {
      if (buffer.length > 0) {
        process.stdout.write(buffer);
        buffer = '';
      }
    },
  };
}

async function runStreamed(model, messages) {
  const streamIter = await model.stream(messages);
  const writer = createLineWriter();
  try {
    for await (const chunk of streamIter) {
      const piece = stringifyContent(chunk.content);
      writer.write(piece);
    }
  } finally {
    writer.end();
  }
}

async function runStructured(baseModel, messages, outputSchema, { stream }) {
  let model = baseModel;
  let effectiveStream = stream;
  if (stream) {
    process.stderr.write(
      '[langchat] warning: -s/--stream ignored: not supported with # !output structured output.\n'
    );
    effectiveStream = false;
  }
  if (effectiveStream !== baseModel.streaming) {
    model = buildModel({
      model: baseModel.model,
      baseURL: baseModel.configuration?.baseURL,
      apiKey: baseModel.apiKey,
      stream: effectiveStream,
    });
  }
  const structured = model.withStructuredOutput(outputSchema);
  return await structured.invoke(messages);
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

  let expanded;
  try {
    expanded = resolveIncludes(text, {
      baseDir: path.dirname(absPath),
      allowEscape: opts.allowIncludeEscape,
    });
  } catch (err) {
    process.stderr.write(`langchat: ${err.message}\n`);
    process.exit(1);
  }

  let parsed;
  try {
    parsed = parseChatFile(expanded.text, expanded.attachments);
  } catch (err) {
    process.stderr.write(`langchat: failed to parse ${opts.file}: ${err.message}\n`);
    process.exit(1);
  }
  const { messages, outputSchema } = parsed;

  let config;
  try {
    config = resolveConfig({ model: opts.model });
  } catch (err) {
    process.stderr.write(`langchat: ${err.message}\n`);
    process.exit(2);
  }

  const model = buildModel({ ...config, stream: opts.stream });

  try {
    if (outputSchema) {
      const result = await runStructured(model, messages, outputSchema, {
        stream: opts.stream,
      });
      process.stdout.write(JSON.stringify(result, null, 2) + '\n');
    } else if (opts.stream) {
      await runStreamed(model, messages);
      process.stdout.write('\n');
    } else {
      const reply = await runOnce(model, messages);
      process.stdout.write(reply.endsWith('\n') ? reply : reply + '\n');
    }
  } catch (err) {
    process.stderr.write(`langchat: request failed: ${err.message}\n`);
    process.exit(1);
  }
}

module.exports = { main, parseArgs, runOnce, runStreamed, runStructured };