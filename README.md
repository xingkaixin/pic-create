# pic-create

`pc` is a minimal Go CLI that generates or edits images through a ChatGPT OAuth session. It reuses the login managed by Codex CLI, so it does not need an API key.

## Installation

```bash
go install github.com/xingkaixin/pic-create/cmd/pc@latest
```

To install from a local checkout instead:

```bash
go install ./cmd/pc
```

## Authentication

Sign in through Codex CLI before using `pc`:

```bash
codex login
```

`pc` reads `~/.codex/auth.json` and refreshes the OAuth session when needed. It preserves the rest of the Codex auth file when writing refreshed tokens.

## Usage

```bash
pc 16:9 "A cinematic city skyline at sunset" -o ./out -n skyline
pc 2.35:1 --prompt-file prompt.txt -n banner.png
pc 9:16 prompt.txt -o ./out -n poster.png
```

The default model is `gpt-image-2`, and the default long edge is `1536` pixels. Output dimensions are scaled proportionally and aligned to multiples of 16 as required by the Image API.

## Edit an Image

```bash
pc edit input.png "Replace the background with a clean white studio backdrop" -o ./out -n edited
pc edit input.png --prompt-file edit-prompt.txt --input-fidelity high -n edited.png
```

Edit mode reads the prompt the same way as generate mode: pass text directly, pass a path as the prompt argument, or use `--prompt-file`.

The command-line arguments remain compatible with the previous API-key version. The ChatGPT OAuth image route currently returns PNG only, so `--format webp` reports an error and `--compression` has no effect with PNG output.
