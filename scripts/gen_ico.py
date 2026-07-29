#!/usr/bin/env python3
"""Generate multi-size Windows .ico from theme/icons/asm2hex.png."""
from __future__ import annotations

import sys
from pathlib import Path

try:
    from PIL import Image
except ImportError:
    print("Pillow is required: pip install pillow", file=sys.stderr)
    sys.exit(1)

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "theme" / "icons" / "asm2hex.png"
DST = ROOT / "theme" / "icons" / "asm2hex.ico"
SIZES = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]


def main() -> None:
    if not SRC.is_file():
        print(f"missing source icon: {SRC}", file=sys.stderr)
        sys.exit(1)
    img = Image.open(SRC).convert("RGBA")
    # Pillow writes multiple sizes when sizes= is provided
    img.save(DST, format="ICO", sizes=SIZES)
    print(f"wrote {DST} ({DST.stat().st_size} bytes)")


if __name__ == "__main__":
    main()
