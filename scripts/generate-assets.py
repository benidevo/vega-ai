#!/usr/bin/env python3
"""
Generate PNG favicons and OG image from SVG sources.

Requirements:
    pip install cairosvg

Usage:
    python3 scripts/generate-assets.py

Outputs (relative to project root):
    static/images/favicon-{16,32,48}x{16,32,48}.png
    static/images/favicon-192x192.png
    static/images/apple-touch-icon.png           (180x180)
    static/landing/assets/images/icon-512x512.png (512x512)

Note: OG/Twitter card images are generated separately via screenshot-og.mjs
"""

import os
import sys

try:
    import cairosvg
except ImportError:
    print("cairosvg not found. Install with:  pip install cairosvg")
    print("macOS: brew install cairo && pip install cairosvg")
    sys.exit(1)

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SVG_DIR = os.path.join(ROOT, "static", "landing", "assets", "images")
PNG_DIR = os.path.join(ROOT, "static", "images")

# (src_dir, svg_name, dst_dir, png_name, width, height)
ASSETS = [
    (SVG_DIR, "favicon-16x16.svg",    PNG_DIR, "favicon-16x16.png",   16,   16),
    (SVG_DIR, "favicon-32x32.svg",    PNG_DIR, "favicon-32x32.png",   32,   32),
    (SVG_DIR, "favicon-48x48.svg",    PNG_DIR, "favicon-48x48.png",   48,   48),
    (SVG_DIR, "apple-touch-icon.svg", PNG_DIR, "apple-touch-icon.png", 180, 180),
    (SVG_DIR, "icon-192x192.svg",     PNG_DIR, "favicon-192x192.png", 192, 192),
    (SVG_DIR, "icon-512x512.svg",     SVG_DIR, "icon-512x512.png",    512, 512),
]


def convert(src_dir, svg_name, dst_dir, png_name, width, height):
    print(f"  {svg_name} → {png_name} ({width}x{height})")
    cairosvg.svg2png(
        url=os.path.join(src_dir, svg_name),
        write_to=os.path.join(dst_dir, png_name),
        output_width=width,
        output_height=height,
    )


if __name__ == "__main__":
    os.makedirs(PNG_DIR, exist_ok=True)
    for args in ASSETS:
        convert(*args)
    print(f"\nDone. Files written to {PNG_DIR}/ and {SVG_DIR}/")
