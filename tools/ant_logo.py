# -*- coding: utf-8 -*-
"""Shared rendering for the Ant Chrome transparent blue line-art logo.

Design: transparent canvas with a white fingerprint body and refined blue
line-art edges.
"""
from __future__ import annotations

import base64
import zlib

from PIL import Image, ImageChops, ImageDraw, ImageFilter

TRANSPARENT = (0, 0, 0, 0)

# Fingerprint body: white fill + blue lines, with a transparent canvas.
ANT_FILL = (248, 251, 255, 255)
ANT_LINE = (29, 78, 216, 255)  # #1D4ED8, distinct from badge palette

# Eye centers/radius at the 128px reference scale.
EYE_DOTS = [(52, 60, 3), (63, 64, 3)]

# Ant silhouette packed as zlib(base64) of 128 little-endian 128-bit rows.
MASK_B64 = (
    "eNrtlTGSHCEMRWkrIHEV6WbcxPhIDh0NzM04CkcgJKCQv7oRdE/scNmqqXrb09LXR9IY831wWsTH18LMbI3hxZ054p80MTHOUeQ75ynCVNlptJMb+/36g/GyGUfnoOHwoJjF5Uq0uLJ5cAtP7s5ke+NhIZA2i1SkUE581aN6cjRnPcolpEc9xZ/1Oa23usoEzepHs+JeD+pXo45ANS7zaFixdPGB8kXV5JHOR0PdHRDUIFfdjRmhGWmUC1JD3nQ7hQJpkOeVq8UF+Kzsq0W9IU83sm80QospJFJmgryQzwzZQaDIi/nMUFx/S303TkGqHJPtkFTNPhlJLwXFvorLVJSrcPUS5mKKeBYWF4p4F8n6++R8hEqDs2vvWRGYGbInJ+GxWQoK/YMBysVdoFw/uNt+58QEZZsLH1eyyehO4bQ4GhG/uPgkvPQUKzd20ycNZu9ccWMwsx3zhpur9BLWBrLtwRgtQopGOrxHP0KxlXQazTDCs4EG2ikJz/AMgdIk2kBsc8i+Ut7jg9sCz47+BcfgIaXV0Q2xoWbxoGY7RlwZs0XdmL9ru6Cbxl4+cGzg78caEI9Q0dj1ukvRhOQWQ1oK2t+XwOzzGrgXBIKXnJ9wTGZkZxxU7R7QS2ANe/1FDG1b8bERsK7W/MkFQWCnzTZFcy/ogHc7nPmNKzbusW/dLu9y8GMf0wd/7Os/3z9Z/+f8AwyfsEk="
)


def load_mask() -> list[list[bool]]:
    packed = zlib.decompress(base64.b64decode(MASK_B64))
    mask: list[list[bool]] = []
    for row_index in range(128):
        value = int.from_bytes(packed[row_index * 16: (row_index + 1) * 16], "little")
        mask.append([bool(value & (1 << x)) for x in range(128)])
    return mask


def _scaled_mask(mask: list[list[bool]], size: int) -> Image.Image:
    if size == 128:
        fill = Image.new("L", (128, 128), 0)
        fill_px = fill.load()
        for y in range(128):
            for x in range(128):
                if mask[y][x]:
                    fill_px[x, y] = 255
        return fill
    small = Image.new("L", (128, 128), 0)
    small_px = small.load()
    for y in range(128):
        for x in range(128):
            if mask[y][x]:
                small_px[x, y] = 255
    return small.resize((size, size), Image.Resampling.LANCZOS)


def draw_ant(canvas: Image.Image, size: int, mask: list[list[bool]]) -> None:
    fill = _scaled_mask(mask, size)
    outline_radius = max(1, size * 2 // 128)
    outline = fill.filter(ImageFilter.MaxFilter(outline_radius * 2 + 1))
    outline = ImageChops.subtract(outline, fill)

    ant_layer = Image.new("RGBA", (size, size), TRANSPARENT)
    ant_layer.paste(ANT_FILL, (0, 0), fill)
    ant_layer.paste(ANT_LINE, (0, 0), outline)
    canvas.alpha_composite(ant_layer)

    draw = ImageDraw.Draw(canvas)
    eye_radius = max(1, size * 3 // 128)
    for ex, ey, _er in EYE_DOTS:
        cx = size * ex // 128
        cy = size * ey // 128
        draw.ellipse((cx - eye_radius, cy - eye_radius, cx + eye_radius, cy + eye_radius), fill=ANT_LINE)


def render_logo(size: int = 128) -> Image.Image:
    mask = load_mask()
    canvas = Image.new("RGBA", (size, size), TRANSPARENT)
    draw_ant(canvas, size, mask)
    return canvas
