#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: $0 [output-directory]"
  exit 0
fi

if (( $# > 1 )); then
  echo "Usage: $0 [output-directory]" >&2
  exit 2
fi

if ! command -v mise >/dev/null 2>&1; then
  echo "mise is required: https://mise.jdx.dev/getting-started.html" >&2
  exit 1
fi

if [[ ! -s "$HOME/.codex/auth.json" ]]; then
  echo "Codex OAuth session not found. Run: codex login" >&2
  exit 1
fi

if (( $# == 1 )); then
  output_directory="$1"
  if [[ -e "$output_directory" ]]; then
    echo "Output path already exists: $output_directory" >&2
    exit 1
  fi
  mkdir -p "$output_directory"
else
  mkdir -p "$repository_root/out"
  output_directory="$(mktemp -d "$repository_root/out/smoke-test.XXXXXX")"
fi

output_directory="$(cd -- "$output_directory" && pwd)"
generated_image="$output_directory/generated.png"
edited_image="$output_directory/edited.png"

cd "$repository_root"

echo "Generating test image..."
mise run pc -- \
  1:1 \
  "Create a clean square test card divided into four equal quadrants: red upper-left, green upper-right, blue lower-left, and yellow lower-right. Put the exact black text PIC CREATE OAUTH TEST in the center. Use flat colors and no gradients." \
  --quality low \
  --long-edge 1024 \
  --output-dir "$output_directory" \
  --name "$(basename "$generated_image")"

echo "Editing generated image..."
mise run pc -- \
  edit \
  "$generated_image" \
  "Keep the quadrant layout and colors unchanged. Replace the center text with the exact text PIC CREATE EDIT OK and add a thick white circular border around the text." \
  --quality low \
  --size 1024x1024 \
  --input-fidelity high \
  --output-dir "$output_directory" \
  --name "$(basename "$edited_image")"

for image in "$generated_image" "$edited_image"; do
  if [[ ! -s "$image" ]]; then
    echo "Expected image was not created: $image" >&2
    exit 1
  fi
done

echo
echo "Smoke test completed. Compare these images:"
echo "  Generated: $generated_image"
echo "  Edited:    $edited_image"

if command -v file >/dev/null 2>&1; then
  file "$generated_image" "$edited_image"
fi
