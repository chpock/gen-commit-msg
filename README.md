# gen-commit-msg

`gen-commit-msg` is a Go CLI that generates Git commit message candidates from
staged changes using OpenCode, then lets you pick the best one in a terminal
UI (or outputs one message in non-interactive mode).

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
- Starts and stops `opencode serve` automatically (with health checks).
- Creates and reuses a dedicated OpenCode agent prompt file.
- Interactive progress view for pipeline steps in TTY mode.
- Subject selection list when multiple candidates are generated.
- Non-interactive mode support (`--subject-max 1`) for scripts/editor callbacks.
- Config via flags and `GCM_*` environment variables.
- Optional output file mode (`--output` / `GCM_OUTPUT`).

## Agent installation and customization

On startup, the tool ensures an OpenCode agent file exists.

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

- Go 1.26+
- `git` available in `PATH`
- `opencode` available in `PATH`
- Run inside a Git repository
- Have staged changes (`git add ...`) before running

## Install

Build from source:

```bash
make build
```

Binary will be created as `./gen-commit-msg`.

## Quick start

Generate up to 5 candidates and choose one:

```bash
gen-commit-msg
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
| `--subject-min`, `-m` | `GCM_SUBJECT_MIN` | `1` | Minimum subject candidate count |
| `--subject-max`, `-x` | `GCM_SUBJECT_MAX` | `5` | Maximum subject candidate count (max 20) |
| `--body` | `GCM_BODY` | `true` | Generate commit body |
| `--quiet`, `-q` | `GCM_QUIET` | `false` | Hide progress view |
| `--agent`, `-a` | `GCM_AGENT` | `gen-commit-msg` | OpenCode agent name |
| `--install-agent` | `GCM_INSTALL_AGENT` | `if-not-exists` | `always`, `if-not-exists`, `no` |
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

Pipeline (interactive mode):

1. Collect staged-change context with multiple Git commands.
2. Ensure OpenCode agent prompt exists.
3. Start OpenCode server and verify readiness.
4. Create OpenCode session.
5. Request structured commit candidates (JSON schema-constrained).
6. Show candidate subjects; user selects one.
7. Cleanup session/server and print (or write) selected message.

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

## Project structure

```text
cmd/gen-commit-msg/main.go    # CLI orchestration
internal/config/              # flags + env parsing
internal/git/                 # repo checks + staged context collection
internal/agent/               # OpenCode agent prompt management
internal/server/              # opencode serve lifecycle
internal/opencode/            # OpenCode session/prompt client
internal/tui/                 # progress + selection Bubble Tea UI
internal/logging/             # slog setup and custom trace level
```

## License

MIT (see `LICENSE`).
