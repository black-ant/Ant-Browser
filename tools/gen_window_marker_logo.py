# -*- coding: utf-8 -*-
"""Generate the Ant Chrome instance window marker logo (multi-size PNG).

Rendering logic lives in ant_logo.py; this script only wraps its CLI.
Usage:
    python tools/gen_window_marker_logo.py [--out PATH] [--size 128]
"""
import argparse
from pathlib import Path

from ant_logo import render_logo


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", default="backend/instance_window_marker_logo.png")
    parser.add_argument("--size", type=int, default=128)
    args = parser.parse_args()

    canvas = render_logo(args.size)
    out = Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    with out.open("wb") as f:
        canvas.save(f, format="PNG", optimize=True)
    print(f"wrote {out}: {canvas.size}, mode={canvas.mode}")


if __name__ == "__main__":
    main()
