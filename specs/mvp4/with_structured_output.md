#!system

You are a help assistant.

#!user

Answer my questions based on the below context:

<context>
{{ include "context.txt" }}
</context>

#!user

Who is mentioned above?

#!output

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "name": {
      "type": "string"
    },
    "sigature attack": {
      "type": "string"
    }
  },
  "required": ["name", "sigature attack"]
}
```