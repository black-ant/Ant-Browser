from __future__ import annotations

import argparse
import io
import struct
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter


DESIGN_SIZE = 1024
BLUE_TOP = (42, 132, 255)
BLUE_BOTTOM = (8, 42, 104)
BLUE_GLOW = (105, 196, 255, 255)
WHITE = (255, 255, 255, 255)
BLACK = (2, 8, 18, 235)


def build_master_icon() -> Image.Image:
    size = DESIGN_SIZE
    canvas = Image.new('RGBA', (size, size), (0, 0, 0, 0))

    shadow = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    shadow_draw = ImageDraw.Draw(shadow)
    shadow_draw.rounded_rectangle((50, 58, 974, 982), radius=230, fill=(0, 0, 0, 160))
    canvas.alpha_composite(shadow.filter(ImageFilter.GaussianBlur(20)))

    outer_mask = Image.new('L', (size, size), 0)
    ImageDraw.Draw(outer_mask).rounded_rectangle((46, 38, 978, 970), radius=230, fill=255)
    gradient = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    gradient_draw = ImageDraw.Draw(gradient)
    for pixel_y in range(size):
        progress = pixel_y / (size - 1)
        red = int(BLUE_TOP[0] * (1 - progress) + BLUE_BOTTOM[0] * progress)
        green = int(BLUE_TOP[1] * (1 - progress) + BLUE_BOTTOM[1] * progress)
        blue = int(BLUE_TOP[2] * (1 - progress) + BLUE_BOTTOM[2] * progress)
        gradient_draw.line((0, pixel_y, size, pixel_y), fill=(red, green, blue, 255))
    canvas.paste(gradient, (0, 0), outer_mask)

    draw = ImageDraw.Draw(canvas)
    draw.rounded_rectangle((64, 56, 960, 954), radius=214, outline=(125, 207, 255, 155), width=8)

    draw.rounded_rectangle((738, 94, 888, 154), radius=30, fill=BLACK)
    draw.ellipse((795, 113, 823, 141), fill=BLUE_GLOW)

    a_points = [(280, 780), (448, 260), (566, 260), (744, 780), (626, 780), (579, 631), (409, 631), (365, 780)]
    draw.polygon(a_points, fill=WHITE)
    draw.polygon([(505, 384), (554, 520), (456, 520)], fill=(12, 54, 126, 255))
    draw.polygon([(403, 558), (594, 558), (620, 626), (380, 626)], fill=(12, 54, 126, 255))
    draw.line((584, 350, 678, 636), fill=BLUE_GLOW, width=14)
    return canvas


def build_ico(frames: list[Image.Image]) -> bytes:
    png_frames: list[bytes] = []
    for frame in frames:
        buffer = io.BytesIO()
        frame.save(buffer, format='PNG', optimize=True)
        png_frames.append(buffer.getvalue())

    header = struct.pack('<HHH', 0, 1, len(png_frames))
    entries = bytearray()
    body = bytearray()
    offset = 6 + 16 * len(png_frames)
    for frame, png_data in zip(frames, png_frames):
        width, height = frame.size
        entries.extend(
            struct.pack(
                '<BBBBHHII',
                width if width < 256 else 0,
                height if height < 256 else 0,
                0,
                0,
                1,
                32,
                len(png_data),
                offset,
            )
        )
        body.extend(png_data)
        offset += len(png_data)
    return header + bytes(entries) + bytes(body)


def save_parent(path: Path, data: bytes | Image.Image) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if isinstance(data, Image.Image):
        data.save(path, format='PNG', optimize=True)
    else:
        path.write_bytes(data)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument('--png', default='build/appicon.png')
    parser.add_argument('--ico', default='build/windows/icon.ico')
    parser.add_argument('--favicon', default='frontend/public/favicon.png')
    parser.add_argument('--preview-dir', default='')
    args = parser.parse_args()

    master = build_master_icon()
    icon_sizes = (16, 20, 24, 32, 40, 48, 64, 128, 256)
    frames = [master.resize((size, size), Image.Resampling.LANCZOS) for size in icon_sizes]

    save_parent(Path(args.png), master)
    save_parent(Path(args.ico), build_ico(frames))
    save_parent(Path(args.favicon), frames[-1])

    if args.preview_dir:
        preview_dir = Path(args.preview_dir)
        preview_dir.mkdir(parents=True, exist_ok=True)
        for size, frame in zip(icon_sizes, frames):
            frame.save(preview_dir / f'app_{size}.png', format='PNG', optimize=True)

    print(f'wrote {args.png}, {args.ico}, {args.favicon}; sizes={icon_sizes}')


if __name__ == '__main__':
    main()
