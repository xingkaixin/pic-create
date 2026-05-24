# pic-create

`pc` is a minimal CLI tool for the OpenAI Image API. It reads an aspect ratio and a prompt, calls `/v1/images/generations`, and saves the result as `png` or `webp`.

## Installation

```bash
uv tool install .
```

## Environment Variables

```bash
export PC_API_KEY="..."
export PC_BASE_URL="https://api.openai.com/v1"
```

`PC_BASE_URL` is optional. When set, it must be a base URL accepted by the OpenAI Python SDK (typically includes `/v1`).

## Usage

```bash
pc 16:9 "A cinematic city skyline at sunset" -o ./out -n skyline
pc 2.35:1 --prompt-file prompt.txt -f webp --compression 80 -n banner.webp
pc 9:16 prompt.txt -o ./out -n poster.png
```

The default model is `gpt-image-2`, and the default long edge is `1536` pixels. Output dimensions are scaled proportionally and aligned to multiples of 16 as required by the Image API.

