---
model: qwen-vl-plus
---

# !system

You are a help assistant.

# !user

This is an image splitted into 4 pieces, tiling from left to right, top to bottom, with 30% overlap.
Piece them together and answer my question

{{ patchify "Goku.png", 2, 2, 50, 50 }}

# !user

What is adult Sun Goku saying?
