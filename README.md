# langchat

A small CLI that turns a Markdown file into a chat completion request against any
OpenAI-compatible `/chat/completions` endpoint and prints the model's reply.

It is built on top of [LangChain.js](https://js.langchain.com/) (`@langchain/openai`)
and is designed to be driven entirely from a `chat.md`-style file, so prompts,
context files, and example images live next to the prompt, not in code.

---

## Installation

```bash
npm install
```

`npm` will fetch `@langchain/openai` and `langchain`. You can then invoke the CLI
through the `langchat` binary (after `npm link`) or directly via `node`:

```bash
node bin/langchat.js path/to/chat.md
# or, after `npm link`
langchat path/to/chat.md
```

## Configuration

`langchat` auto-loads a `.env` file from the current working directory on startup
(existing environment variables always win). See [`.env.example`](.env.example)
for the full template:

| Variable            | Required | Notes                                                                                            |
| ------------------- | -------- | ------------------------------------------------------------------------------------------------ |
| `LANGCHAT_MODEL`    | yes      | Model name sent to `/chat/completions` (e.g. `gpt-4o-mini`, `deepseek-chat`, `llama3.1`).       |
| `LANGCHAT_BASE_URL` | no       | Base URL of an OpenAI-compatible server. Leave unset for OpenAI; set for DeepSeek / Ollama / LM Studio / vLLM. |
| `LANGCHAT_API_KEY`  | no       | Falls back to `OPENAI_API_KEY`. When neither is set, `langchat` warns and uses a placeholder so local servers work without auth. |
| `LANGCHAT_TEMPERATURE` | no    | Sampling temperature. Overridden by `temperature:` in a file header.                             |
| `LANGCHAT_TOP_P`    | no       | Top-p sampling. Overridden by `top_p:` in a file header.                                         |
| `LANGCHAT_MAX_TOKENS` | no     | Max output tokens. Overridden by `max_tokens:` in a file header.                                 |
| `LANGCHAT_TIMEOUT`  | no       | Request timeout in ms. Overridden by `timeout:` in a file header.                                |
| `LANGCHAT_MAX_RETRIES` | no    | Retry count. Overridden by `max_retries:` in a file header.                                       |

Quick examples:

```bash
# OpenAI
export LANGCHAT_MODEL=gpt-4o-mini
export LANGCHAT_API_KEY=sk-...

# DeepSeek
export LANGCHAT_BASE_URL=https://api.deepseek.com/v1
export LANGCHAT_MODEL=deepseek-chat
export LANGCHAT_API_KEY=sk-...

# Ollama (local)
export LANGCHAT_BASE_URL=http://localhost:11434/v1
export LANGCHAT_MODEL=llama3.1

# LM Studio (local)
export LANGCHAT_BASE_URL=http://localhost:1234/v1
export LANGCHAT_MODEL=local-model
```

A reasonable starting point is to copy `.env.example` to `.env` and edit it.

---

## Usage

```
langchat [options] <chat.md>

Options:
  -m, --model <name>         Model name (overrides LANGCHAT_MODEL)
  -s, --stream               Stream the response token-by-token to stdout
  -o, --output <path>        Write the response to <path> as well as stdout
      --allow-include-escape  Permit {{ include }} paths outside the chat file's directory
  -h, --help                 Show this help and exit
```

### Chat file format

A chat file is a Markdown document divided into blocks by headers of the form
`# !<role>`. The supported roles are `system`, `user`, and `assistant`, plus a
special `output` block (see Structured output below). The file must begin with a
role header; everything between two headers is the body of that message.

```markdown
# !system
You are a helpful assistant.

# !user
What's the weather like today?
```

Run it:

```bash
langchat chat.md
```

#### Metadata header

A chat file may optionally begin with a `---`-delimited YAML-style header that
declares per-file options. When present, the header is parsed and stripped from
the body before the `# !<role>` blocks are read; error line numbers from the
parser refer back to the original file.

```markdown
---
model: qwen-vl-plus
streaming: true
temperature: 0.7
max_tokens: 1024
thinking: true
---

# !system
You are a help assistant.

# !user
What is Sun Goku saying?
```

The header also accepts `output:` to write the response to a file (see
[Writing the response to a file](#writing-the-response-to-a-file) below).

**Recognized keys** are mapped to `ChatOpenAI` constructor params:

| Header key         | ChatOpenAI field           |
| ------------------ | -------------------------- |
| `model`            | `model`                    |
| `streaming`        | `streaming`                |
| `temperature`      | `temperature`              |
| `top_p`            | `topP`                     |
| `max_tokens`       | `maxTokens`                |
| `stop`             | `stopSequences`            |
| `presence_penalty` | `presencePenalty`          |
| `frequency_penalty`| `frequencyPenalty`         |
| `timeout`          | `timeout`                  |
| `max_retries`      | `maxRetries`               |

Any other key (e.g. `thinking`, `reasoning_effort`, `seed`) is forwarded to
the API as a field in `modelKwargs`, so the same header works for vendor-
specific extras like Anthropic / DeepSeek reasoning parameters.

Values are parsed as scalars: strings (`foo` or `"foo bar"`), integers (`42`),
floats (`0.7`), booleans (`true` / `false`), and `null` / `~`. Comments
(`# ...`) and blank lines inside the header are ignored.

**Precedence is CLI flag > header > env.** A `-m gpt-4o` flag overrides
`model:` in the header, which in turn overrides `LANGCHAT_MODEL` in the
environment. For the non-model options, the matching env vars are
`LANGCHAT_TEMPERATURE`, `LANGCHAT_TOP_P`, `LANGCHAT_MAX_TOKENS`,
`LANGCHAT_TIMEOUT`, and `LANGCHAT_MAX_RETRIES`. `streaming` is CLI/header
only (no env var).

---

## Examples

The `specs/` directory contains runnable examples, one per milestone.

### MVP1 — Plain text chat

The simplest case: a system prompt and a single user turn. See
[`specs/mvp1/chat.md`](specs/mvp1/chat.md):

```markdown
# !system
You are a help assistant.

# !user
What's the weather like today.
```

```bash
langchat specs/mvp1/chat.md
```

For token-by-token output, pass `-s`/`--stream`. See
[`specs/mvp1/test_streaming.md`](specs/mvp1/test_streaming.md):

```bash
langchat -s specs/mvp1/test_streaming.md
```

### MVP2 — Including a text file as context

Use the `{{ include "path" }}` directive inside any block to splice in the
contents of a text file. Paths are resolved relative to the chat file's
directory and are sandboxed inside it by default (use `--allow-include-escape`
to allow paths that point outside).

[`specs/mvp2/context.txt`](specs/mvp2/context.txt) holds the context and
[`specs/mvp2/with_textfile_context.md`](specs/mvp2/with_textfile_context.md)
references it:

```markdown
# !system
You are a help assistant.

# !user
Answer my questions based on the below context:

<context>
{{ include "context.txt" }}
</context>

# !user
What is Sun Goku's signature attack?
```

```bash
langchat specs/mvp2/with_textfile_context.md
```

The directive is expanded before the file is sent, so the model sees the
context as part of the user message.

### MVP3 — Including an image

When `{{ include "path" }}` points at a supported image extension
(`.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`), `langchat` reads the file as a
base64 attachment and ships it as multimodal content alongside the surrounding
text. Image includes are only valid inside a `# !user` block.

[`specs/mvp3/with_image_context.md`](specs/mvp3/with_image_context.md):

```markdown
---
model: qwen-vl-plus
---

# !system
You are a help assistant.

# !user
Answer my questions based on the below context:
{{ include "Goku.png" }}

# !user
What is Sun Goku saying?
```

```bash
langchat specs/mvp3/with_image_context.md
```

The leading `---` header pins the run to `qwen-vl-plus` so the example is
reproducible without setting `LANGCHAT_MODEL` in the shell.

(`specs/mvp3/Goku.png` is the referenced image.)

### MVP4 — Structured output

Add a single `# !output` block containing a JSON Schema (optionally wrapped in a
fenced code block). `langchat` calls the model with
`withStructuredOutput(schema)`, pretty-prints the returned object as JSON, and
prints it to stdout. Streaming is disabled automatically when a structured
output schema is present.

[`specs/mvp4/with_structured_output.md`](specs/mvp4/with_structured_output.md):

```markdown
# !system
You are a help assistant.

# !user
Answer my questions based on the below context:

<context>
{{ include "context.txt" }}
</context>

# !user
Who is mentioned above?

# !output
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "name":            { "type": "string" },
    "sigature attack": { "type": "string" }
  },
  "required": ["name", "sigature attack"]
}
```
```

```bash
langchat specs/mvp4/with_structured_output.md
```

The model's reply is parsed against the schema and printed as JSON.

### Writing the response to a file

Pass `-o <path>` (or `--output <path>`) to write the response to a file in
addition to stdout. The response is always mirrored to stdout so you can watch
the run interactively; the file just gets a copy. The flag works with plain
text replies, streaming (`-s`), and structured `# !output` results.

```bash
langchat -o reply.md chat.md
langchat -s -o reply.md chat.md       # writes each completed line to reply.md
langchat -o nested/dir/out.md chat.md # creates missing parent dirs (mkdir -p)
```

You can also pin the destination per chat file via the frontmatter header:

```markdown
---
output: results/reply.md
---

# !system
...
```

**Precedence is CLI flag > header > (stdout only).** If both `-o` and `output:`
are set, the CLI flag wins. The file is overwritten on each run; missing parent
directories are created automatically.

---

## Options reference

| Flag                       | Effect                                                                                              |
| -------------------------- | --------------------------------------------------------------------------------------------------- |
| `-m, --model <name>`       | Override `LANGCHAT_MODEL` for this invocation only.                                                 |
| `-s, --stream`             | Stream the response token-by-token to stdout. Automatically disabled when a `# !output` schema is present. |
| `-o, --output <path>`      | Write the response to `<path>` as well as stdout. Creates parent dirs if missing; overwrites existing files. |
| `--allow-include-escape`   | Permit `{{ include "..." }}` paths to escape the chat file's directory. Default is to sandbox them. |
| `-h, --help`               | Print the usage banner and exit.                                                                    |

## Include directive rules

- `{{ include "relative/or/absolute/path" }}`
- Resolved relative to the file that contains the directive (so includes are
  scoped to the chat file's directory).
- A recognized image extension (`.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`) is
  loaded as a base64 attachment and replaces the directive in-place inside a
  `# !user` block.
- Anything else is loaded as UTF-8 text and recursively expanded, so nested
  includes work up to a depth of 8.
- Cyclic includes and paths that escape the chat file's directory are
  rejected. Pass `--allow-include-escape` to opt out of the sandbox.
- Includes larger than 5 MiB are rejected.

## Exit codes

| Code | Meaning                                                       |
| ---- | ------------------------------------------------------------- |
| `0`  | Success.                                                      |
| `1`  | A request, file read, parse, or include failure.              |
| `2`  | Bad usage (missing model, unknown flag, missing chat file).   |

---

## Running the tests

```bash
npm test
```

## License

MIT. See [LICENSE](LICENSE).