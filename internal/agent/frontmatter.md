---
description: Generate JSON Git commit message candidates from supplied staged-change context.
mode: subagent
hidden: true
model: openai/gpt-5.6-luna
top_p: 0.9
temperature: 0.1
steps: 12
reasoningEffort: medium
textVerbosity: low
permission:
  edit: deny
  bash: deny
  webfetch: deny
  websearch: deny
  external_directory: deny
  task: deny
  todowrite: deny
  lsp: deny
  skill: deny
  question: deny
  read: allow
  glob: allow
  grep: allow
  list: allow
---
