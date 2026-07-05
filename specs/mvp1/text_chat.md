implement a cli tool that uses langchain.js to send chat messages to LLM `/chat/completions` endpoint and print the response text to console.

usage:

```bash
langchat <chat.md>
```

where `chat.md` looks like this:


```markdown

# !system

You are a help assistant.

# !user

What's the weather like today.

```

messages are separated by `# !`. There could be multiple user messages mixed with text and images. Only implement text message for now but design it to be extensible later.
