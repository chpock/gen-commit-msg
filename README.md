# gen-commit-msg

`gen-commit-msg` is a Go CLI that generates Git commit message candidates from
staged changes using Pi or OpenCode, then lets you pick the best one in a
terminal UI (or outputs one message in non-interactive mode).

The tool is built for editor-first commit workflows: generate inside the commit
message editor, review, then edit before finalizing the commit.

## Why this tool exists

Many AI commit-message tools are either:

- tied to `prepare-commit-msg` hooks (runs every commit, even when you do not
  want generation), or
- wrappers that run `git commit` for you.

This project intentionally does neither.

- No Git hook integration.
- No commit execution.
- You keep your existing flow (`git commit`, `lazygit`, editor buffer).

`gen-commit-msg` only generates message text and writes it to stdout (or a file
via `--output`).

## Features

- Collects structured staged-change context via Git before prompting AI.
- Automatically prefers Pi when available, or uses OpenCode if Pi is absent.
- Supports explicit `--harness pi` and `--harness opencode` selection.
- Owns runtime startup and cleanup before selection or output.
- Uses a sessionless Pi RPC process with read-only tools and strict JSON checks.
- Starts `opencode serve` and manages its dedicated agent and session.
- Interactive progress view for pipeline steps in TTY mode.
- Subject selection list when multiple candidates are generated.
- Non-interactive mode support (`--subject-max 1`) for scripts/editor callbacks.
- Config via flags and `GCM_*` environment variables.
- Optional output file mode (`--output` / `GCM_OUTPUT`).

## Harness selection and configuration

`--harness auto` (the default) selects `pi` when its executable is in `PATH`,
otherwise `opencode`. If both are installed, Pi is preferred. Use
`--harness opencode` to retain the OpenCode workflow.

Once selected, a harness never falls back to the other one after a version,
authentication, generation, timeout or invalid-response error. Configure and
log in to your chosen harness separately before using this tool.

### Pi

Pi **1.4.0 or newer** is required; older or unrecognizable versions are rejected
before sending Git context. Pi uses its ordinary global model, provider,
thinking, authentication and runtime settings. In Pi, use `/login`, choose a
model with `/model`, and save a default with `Ctrl+S` when needed. Use `/thinking`
and `Ctrl+S` to save its startup thinking level. Compatible endpoints in
`models.json` remain supported; extension-provided models are not available
because extensions are disabled.

**There is no separate model or thinking selection for `gen-commit-msg`.** It
has no application-specific model/thinking flags or environment variables,
passes neither `--model` nor `--thinking` to Pi, and does not change them through
RPC. Each invocation uses Pi's normal startup selection from user settings,
including `defaultProvider`, `defaultModel`, `defaultThinkingLevel`,
`enabledModels` and per-model `modelThinkingLevels`. Pi also limits thinking to
the selected model's capabilities; the effective choice is not determined by
the `default*` fields alone.

The model/thinking currently selected in another open Pi session is not
inherited. Project settings do not affect this selection because they are
disabled for the application-launched Pi process.

The application passes its embedded instructions directly, disables discovered
instructions, extensions/MCP, skills, prompt templates and project settings,
and uses one `--mode rpc --no-session` process. It creates no Pi agent or
persisted conversation. Ordinary Pi configuration side effects, credential
helpers and authentication refresh are not disabled.

Only Pi's native `read`, `grep`, `find` and `ls` tools are enabled:

- `grep` needs cached or system `rg` (ripgrep).
- `find` needs cached or system `fd` (or `fdfind`).
- Pi checks its agent-directory `bin` cache before `PATH`.
- `--offline` disables missing-helper/package/catalog downloads, not model
  requests. Missing helpers produce ordinary tool errors; they do not reject
  application startup or trigger installation or harness fallback. A valid
  final answer after a tool error is still accepted.

Read-only tools **do not enforce repository-only path access**. Repository-only
reading is an instruction, not an operating-system sandbox.

Pi has 30 seconds for version/startup/readiness, 120 seconds for each prompt
operation including native tools/retries/compaction, and a separate five-second
shutdown grace before forced termination. A successful but invalid response
permits at most two corrective prompts in the same conversation. Runtime or
provider failures do not trigger format corrections.

### OpenCode agent installation and customization

When OpenCode is selected, the tool ensures an OpenCode agent file exists.
`--agent`, `--install-agent`, `GCM_AGENT` and `GCM_INSTALL_AGENT` apply only to
OpenCode; Pi ignores them without reading or installing an agent file.

- Default agent name: `gen-commit-msg`
- Default install mode: `if-not-exists`
- Installed path:
  `${XDG_CONFIG_HOME:-$HOME/.config}/opencode/agents/gen-commit-msg.md`

This file contains the agent configuration and generation instructions used for
commit message creation.

You can customize it when needed:

- adjust model settings (for example model name, temperature, steps),
- tune or replace the prompt/instructions for how commit messages are generated.

Important behavior:

- `--install-agent if-not-exists` (default) keeps your custom file intact.
- `--install-agent always` overwrites the file with the built-in default prompt.
- `--install-agent no` skips installation entirely.

## Requirements

- Go 1.26+ to build from source
- `git` available in `PATH`
- Either Pi 1.4.0+ or OpenCode available in `PATH`, with usable model/auth settings
- Run inside a Git repository
- Have staged changes (`git add ...`) before running

## Install

Build from source:

```bash
make build
```

Binary will be created as `./gen-commit-msg`. Install and configure a harness
separately. On Arch Linux, the optional Pi package is
[`pi-coding-agent-bin`](https://aur.archlinux.org/packages/pi-coding-agent-bin);
OpenCode remains an alternative. Search helpers are not mandatory application
dependencies, but must be available for Pi's `grep`/`find` tools to work.

## Quick start

Generate up to 5 candidates and choose one:

```bash
gen-commit-msg
```

Choose a harness explicitly:

```bash
gen-commit-msg --harness pi
gen-commit-msg --harness opencode
```

Generate exactly one candidate (no selection list):

```bash
gen-commit-msg --subject-max 1
```

Write result to a file instead of stdout:

```bash
gen-commit-msg --output /tmp/commit-msg.txt
```

Then use your normal commit command:

```bash
git commit
```

## Editor workflow (Vim example)

This utility is designed to be called from a commit message buffer.

The snippet below runs `gen-commit-msg`, captures output to a temp file, and
inserts the generated message at the top of the current `gitcommit` buffer.

```vim
let s:gen_commit_msg_height = 5
let s:gen_commit_msg_subject_max = 5

function! <SID>GenCommitMsgCallback(orig_bufnr, term_bufnr, tmpfile, job, status)
    if a:status == 0
        if filereadable(a:tmpfile) && getfsize(a:tmpfile) > 0
            let l:output = readfile(a:tmpfile)
            call appendbufline(a:orig_bufnr, 0, l:output)
            call win_execute(bufwinid(a:orig_bufnr), 'call cursor(1, 1)')
        endif
    endif
    call delete(a:tmpfile)
    if a:status == 0 || a:status == 130
        execute 'silent! bwipeout! ' . a:term_bufnr
    else
        echohl WarningMsg | echo 'Command failed with status: ' . a:status | echohl None
    endif
endfunction

function! <SID>GenCommitMsg()
    let l:tmpfile = tempname()
    let l:cmd = 'gen-commit-msg --subject-max ' . shellescape(s:gen_commit_msg_subject_max) . ' --output ' . shellescape(l:tmpfile)
    let l:orig_bufnr = bufnr('%')
    execute 'topleft ' . s:gen_commit_msg_height . 'split | enew'
    let l:Callback = function('<SID>GenCommitMsgCallback', [l:orig_bufnr, bufnr('%'), l:tmpfile])
    call term_start([&shell, &shellcmdflag, l:cmd], {'curwin': 1, 'exit_cb': l:Callback})
endfunction

augroup GitCommitMapping
    autocmd!
    autocmd FileType gitcommit nnoremap <buffer> <Leader>O :call <SID>GenCommitMsg()<CR>
augroup END
```

Press `<leader>O` in a commit buffer to generate candidates and insert the
selected message.

## CLI options

Flag precedence is:

`CLI flag > environment variable > default`

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--harness` | `GCM_HARNESS` | `auto` | `auto`, `pi`, `opencode`; auto prefers Pi |
| `--subject-min`, `-m` | `GCM_SUBJECT_MIN` | `1` | Minimum subject candidate count |
| `--subject-max`, `-x` | `GCM_SUBJECT_MAX` | `5` | Maximum subject candidate count (max 20) |
| `--body` | `GCM_BODY` | `true` | Generate commit body |
| `--quiet`, `-q` | `GCM_QUIET` | `false` | Hide progress, not selection or warnings |
| `--agent`, `-a` | `GCM_AGENT` | `gen-commit-msg` | OpenCode-only agent name |
| `--install-agent` | `GCM_INSTALL_AGENT` | `if-not-exists` | OpenCode-only: `always`, `if-not-exists`, `no` |
| `--pause` | `GCM_PAUSE` | `on-error` | Pause before exit: `on`, `off`, `on-error` |
| `--output`, `-o` | `GCM_OUTPUT` | `""` | Write selected message to file |
| `--log-level`, `-l` | `GCM_LOG_LEVEL` | `none` | `trace`, `debug`, `info`, `warn`, `error`, `none` |
| `--log-file` | `GCM_LOG_FILE` | `stderr` | Log destination (`-` for stdout) |
| `--version`, `-V` | — | — | Print version and exit |
| `--help`, `-h` | — | — | Print help and exit |

Additional runtime env vars:

- `NO_COLOR`: disables selection color styling.
- `GCM_TUI_SELECTION_COLORS`: set to `0` to disable selection color styling.

## How it works

Both interactive and non-interactive runs share one workflow:

1. Check configuration, repository, staged changes and terminal requirements.
2. Resolve the selected harness executable without starting it.
3. Collect staged-change context with multiple Git commands.
4. Run the selected backend:
   - Pi: verify version/readiness, generate and strictly validate candidates,
     perform format corrections only when needed, then stop the RPC process.
   - OpenCode: ensure the agent, start/health-check its server, create a session,
     request native schema-constrained candidates, then delete/stop resources.
5. After cleanup, auto-select a single candidate or show a TTY subject list.
6. Print or write the selected message. Canceling selection leaves output alone.

A cleanup failure after confirmed successful generation is a visible warning,
not loss of the candidates. Warnings use progress or stderr, never the selected
message/file; successful output still exits 0. Cancellation suppresses output.
OpenCode keeps its existing timeout values, with orderly SIGTERM followed by
five seconds of grace and process-group SIGKILL if shutdown remains incomplete.

The staged context includes summaries (`--name-status`, `--stat`, `--numstat`,
`--summary`, `--dirstat`) and full staged patch (`git diff --cached ...`).

## Non-interactive behavior

- If stdout is not a TTY and `--subject-max > 1`: exits with an error and
  suggests `--subject-max 1`.
- If stdout is not a TTY and `--subject-max == 1`: generates silently and prints
  one message (or writes to `--output`).

## Exit codes

- `0`: success (including "no staged files" early exit)
- `1`: runtime error
- `2`: flag/config parsing error

## Development

Common commands:

```bash
make fmt
make vet
make lint
make test
make build
make all
```

Single package tests:

```bash
go test -count=1 -race ./internal/config/
```

Optional actual-Pi integration checks:

```bash
GCM_TEST_NATIVE_PI=1 go test -count=1 -race ./cmd/gen-commit-msg -run '^TestNativePi'
```

These checks use Node preload observation, a disposable Pi configuration and a
local model API, not operator credentials or paid model requests. They require
Node, `rg` and `fd`/`fdfind`. Set `GCM_TEST_NATIVE_PI_PATH` to select the installed
Node-based Pi CLI explicitly. Ordinary `make all` skips them before looking up
native prerequisites. The checks cover real tools, missing helpers without
downloads, instruction isolation, correction and PTY selection/cancellation.

## Project structure

```text
cmd/gen-commit-msg/main.go    # CLI orchestration
internal/config/              # flags + env parsing
internal/harness/             # deterministic executable selection
internal/generation/          # shared messages, instructions and progress
internal/git/                 # repo checks + staged context collection
internal/pi/                  # Pi process, RPC, validation and correction
internal/agent/               # OpenCode agent prompt management
internal/server/              # opencode serve lifecycle
internal/opencode/            # OpenCode session/prompt client
internal/tui/                 # progress + selection Bubble Tea UI
internal/logging/             # slog setup and custom trace level
```

## License

MIT (see `LICENSE`).
