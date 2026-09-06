#!/usr/bin/env python3
"""Regenerate the embedded brand assets (icon.png, favicon.ico) from Tally.png.

Source is the 3D-style illustration at the repo root. This script:
  1. knocks the near-white background out to transparency (flood fill from the
     corners, so the white receipt body in the middle is kept);
  2. trims to the artwork and re-pads with a small margin;
  3. scales to 512x512;
  4. applies a rounded-rectangle alpha mask so the icon is not a hard square.

Run from anywhere; paths are resolved relative to this file.

    python3 mcp/internal/assets/generate.py
"""

from pathlib import Path

from PIL import Image, ImageDraw

HERE = Path(__file__).resolve().parent
SOURCE = HERE.parent.parent.parent / "Tally.png"
ICON_OUT = HERE / "icon.png"
FAVICON_OUT = HERE / "favicon.ico"

SIZE = 512
MARGIN_FRAC = 0.08          # padding around the trimmed artwork
CORNER_RADIUS_FRAC = 0.22   # rounded-rectangle radius, as a fraction of SIZE
BG_FLOOD_THRESHOLD = 32     # colour distance tolerance when flood-filling the bg


def drop_background(img: Image.Image) -> Image.Image:
    """Flood-fill the near-uniform background from each corner to transparent."""
    img = img.convert("RGBA")
    w, h = img.size
    for seed in ((0, 0), (w - 1, 0), (0, h - 1), (w - 1, h - 1)):
        ImageDraw.floodfill(img, seed, (0, 0, 0, 0), thresh=BG_FLOOD_THRESHOLD)
    return img


def trim_and_pad(img: Image.Image) -> Image.Image:
    """Crop to the opaque artwork, then centre it on a padded square canvas."""
    bbox = img.getchannel("A").getbbox()
    if bbox:
        img = img.crop(bbox)
    side = max(img.size)
    pad = int(side * MARGIN_FRAC)
    canvas = Image.new("RGBA", (side + 2 * pad, side + 2 * pad), (0, 0, 0, 0))
    canvas.alpha_composite(
        img,
        ((canvas.width - img.width) // 2, (canvas.height - img.height) // 2),
    )
    return canvas


def rounded(img: Image.Image, size: int) -> Image.Image:
    img = img.resize((size, size), Image.LANCZOS)
    mask = Image.new("L", (size, size), 0)
    ImageDraw.Draw(mask).rounded_rectangle(
        (0, 0, size - 1, size - 1),
        radius=int(size * CORNER_RADIUS_FRAC),
        fill=255,
    )
    out = img.copy()
    alpha = Image.composite(img.getchannel("A"), Image.new("L", (size, size), 0), mask)
    out.putalpha(alpha)
    return out


def main() -> None:
    src = Image.open(SOURCE)
    art = trim_and_pad(drop_background(src))

    icon = rounded(art, SIZE)
    icon.save(ICON_OUT)
    print(f"wrote {ICON_OUT} ({icon.size[0]}x{icon.size[1]})")

    icon.save(FAVICON_OUT, sizes=[(16, 16), (32, 32), (48, 48)])
    print(f"wrote {FAVICON_OUT} (16/32/48)")


if __name__ == "__main__":
    main()
