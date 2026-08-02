<p align="center">
  <img src="docs/images/mahcdown-logo.png" alt="Mahcdown logo" width="200">
</p>

# Mahcdown

vibe trash, kehd. markdown viewah for mac, brah

## Shortcuts

- Esc or Q: quit
- R: reload the file
- Cmd+F: search (opens the find bar, Enter/Shift+Enter navigates)
- Cmd+C: copy highlighted text
- Cmd+= / Cmd+-: zoom in/out

## Local images

Local images are restricted to the Markdown document's directory tree by default. To allow a
document broader access to local image files outside that directory, run:

```text
mahcdown --allow-outside-local-images <path-to-markdown-file>
```

Remote images remain blocked in both modes.
