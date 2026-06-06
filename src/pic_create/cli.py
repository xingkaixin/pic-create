import argparse
import base64
import os
import sys
from collections.abc import Sequence
from pathlib import Path

from openai import OpenAI


MIN_PIXELS = 655_360
MAX_PIXELS = 8_294_400
MAX_EDGE = 3_840
MAX_RATIO = 3
DEFAULT_LONG_EDGE = 1_536


def parse_aspect_ratio(value: str) -> tuple[float, float]:
    parts = value.split(":", 1)
    if len(parts) != 2:
        raise argparse.ArgumentTypeError("aspect ratio must be W:H, e.g. 16:9 or 2.35:1")

    try:
        width = float(parts[0])
        height = float(parts[1])
    except ValueError as exc:
        raise argparse.ArgumentTypeError("aspect ratio must contain only numbers and a colon") from exc

    if width <= 0 or height <= 0:
        raise argparse.ArgumentTypeError("both width and height must be greater than 0")

    ratio = max(width, height) / min(width, height)
    if ratio > MAX_RATIO:
        raise argparse.ArgumentTypeError("OpenAI Image API requires aspect ratio no greater than 3:1")

    return width, height


def round_to_multiple_of_16(value: float) -> int:
    return max(16, round(value / 16) * 16)


def size_from_ratio(aspect_ratio: tuple[float, float], long_edge: int) -> str:
    if long_edge % 16 != 0:
        raise ValueError("--long-edge must be a multiple of 16")
    if long_edge > MAX_EDGE:
        raise ValueError("--long-edge cannot exceed 3840")

    width_ratio, height_ratio = aspect_ratio
    if width_ratio >= height_ratio:
        width = long_edge
        height = round_to_multiple_of_16(long_edge * height_ratio / width_ratio)
    else:
        height = long_edge
        width = round_to_multiple_of_16(long_edge * width_ratio / height_ratio)

    pixels = width * height
    if pixels < MIN_PIXELS:
        raise ValueError(f"computed size {width}x{height} is below the Image API minimum pixel count")
    if pixels > MAX_PIXELS:
        raise ValueError(f"computed size {width}x{height} exceeds the Image API maximum pixel count")

    return f"{width}x{height}"


def read_prompt(prompt: str, prompt_file: str | None) -> str:
    if prompt_file:
        text = Path(prompt_file).read_text(encoding="utf-8").strip()
    else:
        maybe_file = Path(prompt)
        text = maybe_file.read_text(encoding="utf-8").strip() if maybe_file.is_file() else prompt.strip()

    if not text:
        raise ValueError("prompt must not be empty")
    return text


def output_path(directory: str, filename: str, image_format: str) -> Path:
    target_dir = Path(directory).expanduser()
    name = filename if Path(filename).suffix else f"{filename}.{image_format}"
    return target_dir / name


def image_path(value: str) -> Path:
    path = Path(value).expanduser()
    if not path.is_file():
        raise argparse.ArgumentTypeError(f"image file does not exist: {value}")
    return path


def validate_compression(parser: argparse.ArgumentParser, compression: int | None) -> None:
    if compression is not None and not 0 <= compression <= 100:
        parser.error("--compression must be between 0 and 100")


def build_generate_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="pc",
        description="Generate an image through the OpenAI Image API. Use 'pc edit --help' to edit an input image.",
    )
    parser.add_argument("aspect_ratio", type=parse_aspect_ratio, help="aspect ratio, e.g. 16:9, 2.35:1, 9:16")
    parser.add_argument("prompt", nargs="?", help="prompt text; if it is an existing file path, read its content")
    parser.add_argument("-p", "--prompt-file", help="read prompt from a UTF-8 text file")
    parser.add_argument("-o", "--output-dir", default=".", help="output directory, defaults to current directory")
    parser.add_argument("-n", "--name", default="image", help="output filename; may include extension")
    parser.add_argument("-f", "--format", choices=("png", "webp"), default="png", help="output format")
    parser.add_argument("--model", default="gpt-image-2", help="image model, defaults to gpt-image-2")
    parser.add_argument("--quality", choices=("low", "medium", "high", "auto"), default="auto")
    parser.add_argument("--long-edge", type=int, default=DEFAULT_LONG_EDGE, help="long edge in pixels, must be a multiple of 16")
    parser.add_argument("--compression", type=int, default=None, help="WebP compression level 0-100")
    return parser


def build_edit_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="pc edit",
        description="Edit an image through the OpenAI Image API.",
    )
    parser.add_argument("image", type=image_path, help="input image path")
    parser.add_argument("prompt", nargs="?", help="prompt text; if it is an existing file path, read its content")
    parser.add_argument("-p", "--prompt-file", help="read prompt from a UTF-8 text file")
    parser.add_argument("-o", "--output-dir", default=".", help="output directory, defaults to current directory")
    parser.add_argument("-n", "--name", default="image", help="output filename; may include extension")
    parser.add_argument("-f", "--format", choices=("png", "webp"), default="png", help="output format")
    parser.add_argument("--model", default="gpt-image-2", help="image model, defaults to gpt-image-2")
    parser.add_argument("--quality", choices=("low", "medium", "high", "auto"), default="auto")
    parser.add_argument("--size", default="auto", help="output size, defaults to auto")
    parser.add_argument("--input-fidelity", choices=("low", "high"), default=None)
    parser.add_argument("--compression", type=int, default=None, help="WebP compression level 0-100")
    return parser


def client_from_env(parser: argparse.ArgumentParser) -> OpenAI:
    api_key = os.getenv("PC_API_KEY")
    if not api_key:
        parser.error("missing environment variable PC_API_KEY")
    return OpenAI(api_key=api_key, base_url=os.getenv("PC_BASE_URL") or None)


def write_response_image(result: object, target: Path) -> None:
    data = getattr(result, "data", None)
    if not data:
        raise RuntimeError("Image API returned no image data")

    image_base64 = data[0].b64_json
    if not image_base64:
        raise RuntimeError("Image API returned no b64_json")

    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(base64.b64decode(image_base64))


def generate_image(args: argparse.Namespace, parser: argparse.ArgumentParser) -> Path:
    if not args.prompt and not args.prompt_file:
        parser.error("must provide a prompt text or --prompt-file")
    validate_compression(parser, args.compression)

    try:
        prompt = read_prompt(args.prompt or "", args.prompt_file)
        size = size_from_ratio(args.aspect_ratio, args.long_edge)
    except ValueError as exc:
        parser.error(str(exc))

    client = client_from_env(parser)
    request = {
        "model": args.model,
        "prompt": prompt,
        "size": size,
        "quality": args.quality,
        "output_format": args.format,
    }
    if args.format == "webp" and args.compression is not None:
        request["output_compression"] = args.compression

    result = client.images.generate(**request)
    target = output_path(args.output_dir, args.name, args.format)
    write_response_image(result, target)
    return target


def edit_image(args: argparse.Namespace, parser: argparse.ArgumentParser) -> Path:
    if not args.prompt and not args.prompt_file:
        parser.error("must provide a prompt text or --prompt-file")
    validate_compression(parser, args.compression)

    try:
        prompt = read_prompt(args.prompt or "", args.prompt_file)
    except ValueError as exc:
        parser.error(str(exc))

    client = client_from_env(parser)
    request = {
        "image": args.image,
        "model": args.model,
        "prompt": prompt,
        "quality": args.quality,
        "size": args.size,
        "output_format": args.format,
    }
    if args.input_fidelity is not None:
        request["input_fidelity"] = args.input_fidelity
    if args.format == "webp" and args.compression is not None:
        request["output_compression"] = args.compression

    result = client.images.edit(**request)
    target = output_path(args.output_dir, args.name, args.format)
    write_response_image(result, target)
    return target


def main(argv: Sequence[str] | None = None) -> None:
    args_list = list(sys.argv[1:] if argv is None else argv)
    if args_list[:1] == ["edit"]:
        parser = build_edit_parser()
        args = parser.parse_args(args_list[1:])
        target = edit_image(args, parser)
    else:
        parser = build_generate_parser()
        args = parser.parse_args(args_list)
        target = generate_image(args, parser)

    print(target)
