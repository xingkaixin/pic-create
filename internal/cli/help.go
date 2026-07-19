package cli

const generateHelp = `usage: pc [-h] [-p PROMPT_FILE] [-o OUTPUT_DIR] [-n NAME] [-f {png,webp}]
          [--model MODEL] [--quality {low,medium,high,auto}]
          [--long-edge LONG_EDGE] [--compression COMPRESSION]
          aspect_ratio [prompt]

Generate an image through ChatGPT OAuth. Use 'pc edit --help' to edit an input image.

positional arguments:
  aspect_ratio          aspect ratio, e.g. 16:9, 2.35:1, 9:16
  prompt                prompt text; if it is an existing file path, read its content

options:
  -h, --help            show this help message and exit
  -p, --prompt-file     read prompt from a UTF-8 text file
  -o, --output-dir      output directory, defaults to current directory
  -n, --name            output filename; may include extension
  -f, --format          png or webp (OAuth currently supports png only)
  --model               image model, defaults to gpt-image-2
  --quality             low, medium, high, or auto
  --long-edge           long edge in pixels, must be a multiple of 16
  --compression         WebP compression level 0-100
`

const editHelp = `usage: pc edit [-h] [-p PROMPT_FILE] [-o OUTPUT_DIR] [-n NAME] [-f {png,webp}]
               [--model MODEL] [--quality {low,medium,high,auto}]
               [--size SIZE] [--input-fidelity {low,high}]
               [--compression COMPRESSION]
               image [prompt]

Edit an image through ChatGPT OAuth.

positional arguments:
  image                 input image path
  prompt                prompt text; if it is an existing file path, read its content

options:
  -h, --help            show this help message and exit
  -p, --prompt-file     read prompt from a UTF-8 text file
  -o, --output-dir      output directory, defaults to current directory
  -n, --name            output filename; may include extension
  -f, --format          png or webp (OAuth currently supports png only)
  --model               image model, defaults to gpt-image-2
  --quality             low, medium, high, or auto
  --size                output size, defaults to auto
  --input-fidelity      low or high
  --compression         WebP compression level 0-100
`
