# -*- coding: utf-8 -*-
"""Generate the Ant Chrome tray icon (multi-size ICO, PNG-compressed frames).

Design: single ant avatar tuned for small tray sizes (16-64px).
Draws each frame directly at its target size for pixel-true edges.
Usage:
    python tools/gen_tray_icon.py [--out PATH] [--preview DIR] [--sizes 16,20,24,32,40,48,64]
"""
import argparse
import io
import struct

from PIL import Image, ImageDraw

# ---- palette -----------------------------------------------------------------
BG_TOP = (24, 54, 102)       # #183666
BG_BOTTOM = (9, 28, 60)      # #091C3C
BG_RIM = (96, 160, 230)      # inner rim highlight
ANT = (46, 218, 255)         # main cyan
ANT_HL = (170, 246, 255)     # head highlight
EYE = (8, 30, 62)            # dark navy eyes
BADGE_BG = (5, 9, 20)          # near-black badge disc
OUTLINE = (22, 66, 102)        # steel-blue ant outline (sampled from logo)
TRANSPARENT = (0, 0, 0, 0)


def draw_background(img, size):
    s = size
    d = ImageDraw.Draw(img)
    bg_inset = max(0.6, 0.05 * s)
    bg_radius = max(3.2, 0.225 * s)
    bg_box = (bg_inset, bg_inset, s - bg_inset, s - bg_inset)
    grad = Image.new("RGBA", (s, s), TRANSPARENT)
    gd = ImageDraw.Draw(grad)
    for y in range(s):
        t = y / max(s - 1, 1)
        r = int(BG_TOP[0] + (BG_BOTTOM[0] - BG_TOP[0]) * t)
        g = int(BG_TOP[1] + (BG_BOTTOM[1] - BG_TOP[1]) * t)
        b = int(BG_TOP[2] + (BG_BOTTOM[2] - BG_TOP[2]) * t)
        gd.line((0, y, s, y), fill=(r, g, b, 255))
    mask = Image.new("L", (s, s), 0)
    ImageDraw.Draw(mask).rounded_rectangle(bg_box, radius=bg_radius, fill=255)
    img.paste(grad, (0, 0), mask)
    rim_inset = bg_inset + max(0.7, 0.03 * s)
    rim_alpha = 55 if s <= 20 else (110 if s <= 32 else 75)
    d.rounded_rectangle(
        (rim_inset, rim_inset, s - rim_inset, s - rim_inset),
        radius=max(bg_radius - max(0.7, 0.03 * s), 1.0),
        outline=BG_RIM + (rim_alpha,),
        width=1,
    )


def line_round(d, xy, width, fill):
    d.line(xy, fill=fill, width=int(width), joint="curve")
    r = width / 2.0
    for (px_, py_) in ((xy[0], xy[1]), (xy[2], xy[3])):
        d.ellipse((px_ - r, py_ - r, px_ + r, py_ + r), fill=fill)


def draw_badge(d, size):
    """Top-right corner badge: black disc + cyan ring + mini ant head.

    Keeps the main ant full-size (lower 2/3) and puts the project
    identity in the top-right corner, Chrome-taskbar-badge style.
    """
    s = size
    if s <= 20:
        # pixel-exact mini badge (no ring at 16px: it eats the whole disc)
        if s <= 16:
            d.ellipse((11.3, 0.7, 15.3, 4.7), fill=BADGE_BG)
            d.rectangle((12, 1, 12, 1), fill=ANT)
            d.rectangle((14, 1, 14, 1), fill=ANT)
            d.rectangle((12, 2, 12, 2), fill=ANT)
            d.rectangle((14, 2, 14, 2), fill=ANT)
        else:
            d.ellipse((14.3, 0.7, 19.3, 5.7), fill=BADGE_BG)
            d.ellipse((14.8, 1.2, 18.8, 5.2), outline=ANT, width=1)
            d.rectangle((15, 1, 15, 1), fill=ANT)
            d.rectangle((18, 1, 18, 1), fill=ANT)
            d.rectangle((15, 2, 15, 2), fill=ANT)
            d.rectangle((18, 2, 18, 2), fill=ANT)
        return

    b = max(4.5, 0.26 * s)
    inset = max(0.7, 0.05 * s)
    x0, y0 = s - inset - b, inset
    x1, y1 = x0 + b, y0 + b
    bcx, bcy = (x0 + x1) / 2, (y0 + y1) / 2

    # black disc + 1px cyan ring
    d.ellipse((x0, y0, x1, y1), fill=BADGE_BG)
    ring = max(0.4, b * 0.09)
    d.ellipse((x0 + ring, y0 + ring, x1 - ring, y1 - ring), outline=ANT, width=1)

    # eyes (cyan dots on the black head)
    er = max(0.7, 0.075 * b)
    ey = bcy + 0.05 * b
    for side in (-1, 1):
        ex = bcx + side * 0.16 * b
        d.ellipse((ex - er, ey - er, ex + er, ey + er), fill=ANT)

    # antennae
    aw = max(0.8, 0.09 * b)
    for side in (-1, 1):
        ax0_, ay0_ = bcx + side * 0.11 * b, bcy - 0.18 * b
        ax1_, ay1_ = bcx + side * 0.27 * b, bcy - 0.42 * b
        line_round(d, (ax0_, ay0_, ax1_, ay1_), aw, ANT)


def outlined_ellipse(d, box, ow):
    d.ellipse((box[0] - ow, box[1] - ow, box[2] + ow, box[3] + ow), fill=OUTLINE)
    d.ellipse(box, fill=ANT)


def outlined_line(d, xy, width, ow):
    line_round(d, xy, width + 2 * ow, OUTLINE)
    line_round(d, xy, width, ANT)


def alpha_sharpen(img, size):
    """Push faint AA pixels to solid/transparent so edges read crisp at
    small sizes; keep full smoothness at 40px and above."""
    s = size
    if s >= 40:
        return img
    lo, hi = (70, 200) if s <= 20 else (45, 230)
    a = img.getchannel("A")

    def curve(v):
        if v <= lo:
            return 0
        if v >= hi:
            return 255
        t = (v - lo) / (hi - lo)
        t = t * t * (3 - 2 * t)
        return int(t * 255)

    img.putalpha(a.point(curve))
    return img


def draw_full(size):
    """Full-detail ant (3 segments, 6 legs) for 24px and up."""
    s = size
    img = Image.new("RGBA", (s, s), TRANSPARENT)
    draw_background(img, s)
    d = ImageDraw.Draw(img)
    cx = 0.5 * s

    # antennae
    ant_w = max(1.1, 0.072 * s)
    head_cy, head_r = 0.32 * s, 0.155 * s
    ow = max(0.8, 0.03 * s) if s >= 24 else 0.0
    outline_lines = s >= 32
    for side in (-1, 1):
        ax0 = cx + side * 0.05 * s
        ay0 = head_cy - 0.125 * s
        ax1 = cx + side * 0.15 * s
        ay1 = head_cy - 0.225 * s
        if outline_lines:
            outlined_line(d, (ax0, ay0, ax1, ay1), ant_w, ow)
            tip_r = max(0.9, 0.033 * s)
            d.ellipse((ax1 - tip_r - ow, ay1 - tip_r - ow,
                       ax1 + tip_r + ow, ay1 + tip_r + ow), fill=OUTLINE)
            d.ellipse((ax1 - tip_r, ay1 - tip_r, ax1 + tip_r, ay1 + tip_r), fill=ANT)
        else:
            line_round(d, (ax0, ay0, ax1, ay1), ant_w, ANT)
            tip_r = max(0.9, 0.033 * s)
            d.ellipse((ax1 - tip_r, ay1 - tip_r, ax1 + tip_r, ay1 + tip_r), fill=ANT)

    # head + highlight
    outlined_ellipse(d, (cx - head_r, head_cy - head_r, cx + head_r, head_cy + head_r), ow)
    hl_r = 0.042 * s
    d.ellipse((cx - 0.058 * s - hl_r, head_cy - 0.09 * s - hl_r,
               cx - 0.058 * s + hl_r, head_cy - 0.09 * s + hl_r), fill=ANT_HL)

    # eyes (small enough to stay two separated dots at 24px+)
    eye_dx = 0.07 * s
    eye_r = max(0.9, 0.04 * s)
    eye_y = head_cy + 0.012 * s
    for side in (-1, 1):
        d.ellipse((cx + side * eye_dx - eye_r, eye_y - eye_r,
                   cx + side * eye_dx + eye_r, eye_y + eye_r), fill=EYE)

    # legs
    leg_w = max(1.25, 0.082 * s)
    legs = [
        (0.09, 0.485, 0.215, 0.615),
        (0.105, 0.545, 0.235, 0.725),
        (0.12, 0.605, 0.255, 0.845),
    ]
    for dx0, dy0, dx1, dy1 in legs:
        for side in (-1, 1):
            if outline_lines:
                outlined_line(d, (cx + side * dx0 * s, dy0 * s,
                                  cx + side * dx1 * s, dy1 * s), leg_w, ow)
            else:
                line_round(d, (cx + side * dx0 * s, dy0 * s,
                               cx + side * dx1 * s, dy1 * s), leg_w, ANT)

    # thorax
    tx, ty, trx, try_ = cx, 0.535 * s, 0.115 * s, 0.088 * s
    outlined_ellipse(d, (tx - trx, ty - try_, tx + trx, ty + try_), ow)
    # abdomen
    ax, ay, arx, ary = cx, 0.735 * s, 0.185 * s, 0.162 * s
    outlined_ellipse(d, (ax - arx, ay - ary, ax + arx, ay + ary), ow)

    draw_badge(d, s)
    return alpha_sharpen(img, s)


def draw_small(size):
    """Bold minimal ant (big head + single body, 4/6 legs) for 16/20px."""
    s = size
    img = Image.new("RGBA", (s, s), TRANSPARENT)
    draw_background(img, s)
    d = ImageDraw.Draw(img)
    cx = 0.5 * s
    k = s / 16.0
    legs_n = 4 if s <= 16 else 6

    head_cy, head_r = (4.7 * k, 2.4 * k) if s <= 16 else (4.2 * k, 2.4 * k)
    body_cy, body_rx, body_ry = (10.8 * k, 2.7 * k, 3.4 * k) if s <= 16 else (10.3 * k, 2.7 * k, 3.4 * k)

    # antennae
    ant_w = max(1.1, 1.15 * k)
    for side in (-1, 1):
        line_round(d, (cx + side * 0.8 * k, head_cy - 1.6 * k,
                       cx + side * 2.1 * k, head_cy - 2.6 * k), ant_w, ANT)

    # body (single oval, drawn before head so head overlaps the neck)
    d.ellipse((cx - body_rx, body_cy - body_ry, cx + body_rx, body_cy + body_ry), fill=ANT)

    # legs
    leg_w = max(1.25, 1.3 * k)
    if legs_n == 4:
        legs = [
            (1.5, -3.0, 3.4, -0.7),
            (1.7, -1.6, 3.7, 2.2),
        ]
    else:
        legs = [
            (1.5, -2.8, 3.4, -0.6),
            (1.7, -1.6, 3.7, 1.4),
            (1.9, -0.4, 4.0, 3.4),
        ]
    for dx0, dy0, dx1, dy1 in legs:
        for side in (-1, 1):
            line_round(d, (cx + side * dx0 * k, body_cy + dy0 * k,
                           cx + side * dx1 * k, body_cy + dy1 * k), leg_w, ANT)

    # head
    d.ellipse((cx - head_r, head_cy - head_r, cx + head_r, head_cy + head_r), fill=ANT)

    # eyes (keep small so they read as dots, not a dark band)
    eye_y = head_cy + 0.5 * k
    if s <= 16:
        # pixel-exact 1px eyes: crisp separation at 16px
        d.rectangle((6, 5, 6, 5), fill=EYE)
        d.rectangle((9, 5, 9, 5), fill=EYE)
    elif s <= 20:
        # 2x2 pixel eyes on a 6px head
        d.rectangle((8, 5, 9, 6), fill=EYE)
        d.rectangle((11, 5, 12, 6), fill=EYE)
    else:
        eye_dx, eye_r = 1.5 * k, 1.0 * k
        for side in (-1, 1):
            d.ellipse((cx + side * eye_dx - eye_r, eye_y - eye_r,
                       cx + side * eye_dx + eye_r, eye_y + eye_r), fill=EYE)

    draw_badge(d, s)
    return alpha_sharpen(img, s)


def draw_avatar(size):
    """Draw one centered ant avatar without a secondary corner badge."""
    s = size
    img = Image.new("RGBA", (s, s), TRANSPARENT)
    draw_background(img, s)
    d = ImageDraw.Draw(img)
    cx = 0.5 * s
    head_cy = 0.56 * s
    head_r = 0.28 * s
    outline_width = max(1.0, 0.045 * s)
    antenna_width = max(1.0, 0.055 * s)

    for side in (-1, 1):
        start = (cx + side * 0.11 * s, head_cy - 0.2 * s)
        end = (cx + side * 0.27 * s, head_cy - 0.42 * s)
        if s >= 24:
            outlined_line(d, (*start, *end), antenna_width, outline_width)
            tip_radius = max(0.9, 0.035 * s)
            d.ellipse(
                (end[0] - tip_radius - outline_width, end[1] - tip_radius - outline_width,
                 end[0] + tip_radius + outline_width, end[1] + tip_radius + outline_width),
                fill=OUTLINE,
            )
            d.ellipse(
                (end[0] - tip_radius, end[1] - tip_radius,
                 end[0] + tip_radius, end[1] + tip_radius),
                fill=ANT,
            )
        else:
            line_round(d, (*start, *end), antenna_width, ANT)

    head_box = (cx - head_r, head_cy - head_r, cx + head_r, head_cy + head_r)
    if s >= 24:
        outlined_ellipse(d, head_box, outline_width)
        highlight_radius = max(1.0, 0.045 * s)
        highlight_x = cx - 0.08 * s
        highlight_y = head_cy - 0.1 * s
        d.ellipse(
            (highlight_x - highlight_radius, highlight_y - highlight_radius,
             highlight_x + highlight_radius, highlight_y + highlight_radius),
            fill=ANT_HL,
        )
    else:
        d.ellipse(head_box, fill=ANT)

    eye_y = head_cy + 0.015 * s
    if s <= 16:
        d.rectangle((6, 9, 6, 9), fill=EYE)
        d.rectangle((9, 9, 9, 9), fill=EYE)
    elif s <= 20:
        eye_radius = 0.8 * s / 16
        eye_dx = 1.5 * s / 16
        for side in (-1, 1):
            eye_x = cx + side * eye_dx
            d.ellipse(
                (eye_x - eye_radius, eye_y - eye_radius,
                 eye_x + eye_radius, eye_y + eye_radius),
                fill=EYE,
            )
    else:
        eye_radius = max(1.0, 0.04 * s)
        eye_dx = 0.07 * s
        for side in (-1, 1):
            eye_x = cx + side * eye_dx
            d.ellipse(
                (eye_x - eye_radius, eye_y - eye_radius,
                 eye_x + eye_radius, eye_y + eye_radius),
                fill=EYE,
            )

    return alpha_sharpen(img, s)


def draw_ant(size):
    return draw_avatar(size)


def build_ico(frames):
    pngs = []
    for im in frames:
        buf = io.BytesIO()
        im.save(buf, format="PNG")
        pngs.append(buf.getvalue())
    count = len(pngs)
    header = struct.pack("<HHH", 0, 1, count)
    entries = b""
    offset = 6 + 16 * count
    body = b""
    for png in pngs:
        w = struct.unpack(">I", png[16:20])[0]
        h = struct.unpack(">I", png[20:24])[0]
        entries += struct.pack(
            "<BBBBHHII", w if w < 256 else 0, h if h < 256 else 0, 0, 0, 1, 32, len(png), offset
        )
        body += png
        offset += len(png)
    return header + entries + body


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default="backend/internal/tray/icon.ico")
    ap.add_argument("--preview", default=None, help="dir to write per-size PNG previews")
    ap.add_argument("--sizes", default="16,20,24,32,40,48,64")
    args = ap.parse_args()

    sizes = [int(x) for x in args.sizes.split(",")]
    frames = [draw_ant(s) for s in sizes]
    if args.preview:
        import os
        os.makedirs(args.preview, exist_ok=True)
        for s, im in zip(sizes, frames):
            im.save(os.path.join(args.preview, f"tray_{s}.png"))
    data = build_ico(frames)
    with open(args.out, "wb") as f:
        f.write(data)
    print(f"wrote {args.out}: {len(data)} bytes, sizes={sizes}")


if __name__ == "__main__":
    main()
