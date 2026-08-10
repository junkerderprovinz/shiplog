#!/usr/bin/env python3
"""Flood-fill the border-connected background colour of a PNG to transparent,
in place. Keeps any region enclosed by a different colour (e.g. the white
disk inside a dark ring/frame) opaque, since it's never border-connected.

Usage: python3 flood-transparent.py <path-to-png>
"""
import sys
from collections import deque
from PIL import Image


def flood_outer_transparent(im, tol=32):
    im = im.convert("RGBA")
    w, h = im.size
    px = im.load()
    bg = px[0, 0]

    def match(c):
        return c[3] > 0 and abs(c[0] - bg[0]) <= tol and abs(c[1] - bg[1]) <= tol and abs(c[2] - bg[2]) <= tol

    seen = bytearray(w * h)
    dq = deque()
    for x in range(w):
        dq.append((x, 0)); dq.append((x, h - 1))
    for y in range(h):
        dq.append((0, y)); dq.append((w - 1, y))
    while dq:
        x, y = dq.pop()
        i = y * w + x
        if seen[i]:
            continue
        seen[i] = 1
        c = px[x, y]
        if not match(c):
            continue
        px[x, y] = (c[0], c[1], c[2], 0)
        if x > 0: dq.append((x - 1, y))
        if x < w - 1: dq.append((x + 1, y))
        if y > 0: dq.append((x, y - 1))
        if y < h - 1: dq.append((x, y + 1))
    return im


if __name__ == "__main__":
    path = sys.argv[1]
    flood_outer_transparent(Image.open(path)).save(path)
