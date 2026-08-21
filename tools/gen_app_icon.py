from __future__ import annotations

import argparse
import io
import struct
from pathlib import Path

from PIL import Image

from ant_logo import render_logo


DESIGN_SIZE = 1024
ICON_SIZES = (16, 20, 24, 32, 40, 48, 64, 128, 256)


def build_master_icon() -> Image.Image:
    return render_logo(DESIGN_SIZE)


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
    frames = [render_logo(size) for size in ICON_SIZES]

    save_parent(Path(args.png), master)
    save_parent(Path(args.ico), build_ico(frames))
    save_parent(Path(args.favicon), frames[-1])

    if args.preview_dir:
        preview_dir = Path(args.preview_dir)
        preview_dir.mkdir(parents=True, exist_ok=True)
        for size, frame in zip(ICON_SIZES, frames):
            frame.save(preview_dir / f'app_{size}.png', format='PNG', optimize=True)

    print(f'wrote {args.png}, {args.ico}, {args.favicon}; sizes={ICON_SIZES}')


if __name__ == '__main__':
    main()
