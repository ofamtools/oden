#!/usr/bin/env python3
"""Write oden's logo set: assets/icon.svg, assets/logo-light.svg and
assets/logo-dark.svg.

The mark is the Kushi: konnyaku, daikon and chikuwa on one bamboo skewer,
as subcommands sit on one bin. The set follows ochazuke's and matomeru's
assets/: a 128x128 icon on a rounded tile (rx 28) in a dark tint of the
tool's hue, and a logo of the untiled mark beside the name in the family's
monoline lettering, drawn as paths so no font is needed. Standard library
only, except --preview.

Regenerate from the repository root:

    python3 docs/logo/build.py                 # writes assets/*.svg
    python3 docs/logo/build.py --check         # the glyphs still redraw ochazuke's and matomeru's wordmarks
    python3 docs/logo/build.py --preview DIR   # also renders 16, 32 and 128 px PNGs
                                               # and contact sheets (needs rsvg-convert, magick)
"""
import pathlib

import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
ASSETS = ROOT / "assets"
NAME = "oden"

# Dashi amber. TILE is the icon's ground and the light logo's lettering;
# PALE is the dark logo's lettering.
TILE, PALE = "#241a12", "#fff1d6"

# Ingredients (cartoon style: round shapes, a dark gap outline, light detail).
OUTLINE = "#3b2615"
KON, KON_DK, KON_LT = "#9d93ab", "#5f566b", "#c4bccf"
DAI_TOP, DAI_RIM, DAI_SIDE, DAI_LINE = "#fff4dc", "#f0d6a0", "#e7ad5e", "#e2bb7c"
CHK_SKIN, CHK_TOAST, CHK_END, CHK_HOLE = "#c9803f", "#8f4f22", "#fbe6c2", "#3b2615"
# The stick: pale bamboo on dark grounds, a deeper bamboo on white so it reads.
STICK = {"dark": "#dcbc7e", "light": "#c49a52"}


def g(x, y, body, s=1.0, rot=0.0):
    return f'<g transform="translate({x} {y}) rotate({rot}) scale({s})">{body}</g>'


def konnyaku(x, y, s=1.0, rot=0.0):
    body = (f'<path d="M-4.5 -14.5Q0 -20 4.5 -14.5Q13 -3 17 8Q20 16 11 16.5Q0 18 -11 16.5'
            f'Q-20 16 -17 8Q-13 -3 -4.5 -14.5Z" fill="{KON}" stroke="{OUTLINE}" stroke-width="3" stroke-linejoin="round"/>'
            f'<path d="M-7 -1L5 11M-1 -9L11 3M7 -3L-5 9M1 -10L-11 2" stroke="{KON_LT}" stroke-width="1.4" stroke-linecap="round" opacity=".45"/>'
            f'<g fill="{KON_DK}"><circle cx="-9" cy="10" r="1.4"/><circle cx="-2" cy="4" r="1.2"/>'
            f'<circle cx="8" cy="11" r="1.4"/><circle cx="3" cy="-5" r="1.1"/><circle cx="-5" cy="-3" r="1"/>'
            f'<circle cx="12" cy="6" r="1.1"/><circle cx="1" cy="12" r="1.2"/></g>'
            f'<path d="M-8 -3Q-4 -10 -1 -13" fill="none" stroke="#ddd6e5" stroke-width="2.2" stroke-linecap="round"/>')
    return g(x, y, body, s, rot)


def daikon(x, y, s=1.0, rot=0.0, rx=18, ry=7.5, h=12, stick="#dcbc7e"):
    """A thick round slice seen from slightly above: creamy top, chamfered rim, broth-tinted side."""
    top = -h / 2
    side = (f'<path d="M{-rx} {top}V{top+h}A{rx} {ry} 0 0 0 {rx} {top+h}V{top}Z" fill="{DAI_SIDE}" '
            f'stroke="{OUTLINE}" stroke-width="3" stroke-linejoin="round"/>')
    face = (f'<ellipse cx="0" cy="{top}" rx="{rx}" ry="{ry}" fill="{DAI_TOP}" stroke="{OUTLINE}" stroke-width="3"/>'
            f'<ellipse cx="0" cy="{top}" rx="{rx*0.72:.1f}" ry="{ry*0.68:.1f}" fill="none" stroke="{DAI_RIM}" stroke-width="2"/>'
            f'<path d="M{-rx*0.42:.1f} {top-ry*0.2:.1f}L{-rx*0.15:.1f} {top-ry*0.07:.1f}M{rx*0.42:.1f} {top+ry*0.2:.1f}L{rx*0.15:.1f} {top+ry*0.07:.1f}'
            f'M{rx*0.1:.1f} {top-ry*0.45:.1f}L{rx*0.04:.1f} {top-ry*0.18:.1f}M{-rx*0.1:.1f} {top+ry*0.45:.1f}L{-rx*0.04:.1f} {top+ry*0.18:.1f}" '
            f'stroke="{DAI_LINE}" stroke-width="1.5" stroke-linecap="round"/>'
            f'<path d="M{-rx+3} {top+3}Q{-rx+4} {top+h-1} {-rx+9} {top+h+ry*0.55:.1f}" fill="none" stroke="#f6c98a" stroke-width="2" stroke-linecap="round"/>')
    stub = f'<ellipse cx="0" cy="{top}" rx="2.6" ry="1.6" fill="{stick}"/>'
    return g(x, y, side + face + stub, s, rot)


def chikuwa(x, y, s=1.0, rot=0.0):
    """A browned tube lying across the skewer, its pale cut end and hole on the right."""
    body = (f'<path d="M-16 -9H14A6 9 0 0 1 14 9H-16A6 9 0 0 1 -16 -9Z" fill="{CHK_SKIN}" stroke="{OUTLINE}" stroke-width="3" stroke-linejoin="round"/>'
            f'<path d="M-15 -4Q-8 -7 -1 -4Q6 -1 12 -5" fill="none" stroke="{CHK_TOAST}" stroke-width="3.5" stroke-linecap="round"/>'
            f'<ellipse cx="14" cy="0" rx="6" ry="9" fill="{CHK_END}" stroke="{OUTLINE}" stroke-width="2.5"/>'
            f'<ellipse cx="14" cy="0" rx="2.6" ry="4.4" fill="{CHK_HOLE}"/>')
    return g(x, y, body, s, rot)


def mark(theme):
    """The Kushi in the 128x128 icon space."""
    st = STICK[theme]
    return (f'<g transform="rotate(-9 64 64)">'
            f'<path d="M64 8L64 120" stroke="{st}" stroke-width="4.5" stroke-linecap="round"/>'
            f'<circle cx="64" cy="8" r="1.4" fill="#f4dfae"/>'
            + konnyaku(64, 35, 1.25, 4) + daikon(64, 71, 1.28, -5, stick=st) + chikuwa(64, 100, 1.22, 6)
            + '</g>')


# The mark's drawn box in icon space is x 36.7..95.9, y 6.4..121.6 (measured
# from a 1280 px render). In the logo it is scaled to 90 high and set 4 from
# the left, 3 from the top, in the 96-unit band the family's marks share.
LOGO_MARK = "translate(-24.6 -2) scale(0.78)"
MARK_RIGHT = 50.2


# --- The family lettering ---------------------------------------------------
# The glyph table is o-proj's and o-inventory's (docs/logo/build.py in each),
# whose shapes are copied from the ochazuke and matomeru logos; d is o's bowl
# with h's ascender stem. Centre-line strokes 11 wide with round caps and
# joins, x-height y 44..84, ascenders from y 18, 15 between letters (centre
# line to centre line). Every family logo is 112 high (room for descenders),
# so the four READMEs' logos line up; README width = 360 * viewBox width / 518,
# ochazuke's letter height.
STROKE, GAP, HEIGHT = 11, 15, 112


def f(v):
    s = f"{v:.1f}"
    return s[:-2] if s.endswith(".0") else s


GLYPHS = {
    "o": (40, lambda x: f"M{f(x+20)} 44a20 20 0 1 0 0.01 0"),
    "c": (34.1, lambda x: f"M{f(x+34.1)} 49.9A20 20 0 1 0 {f(x+34.1)} 78.1"),
    "h": (36, lambda x: f"M{f(x)} 18V84M{f(x)} 62a18 18 0 0 1 36 0V84"),
    "a": (40, lambda x: f"M{f(x+20)} 44a20 20 0 1 0 0.01 0M{f(x+40)} 44V84"),
    "d": (40, lambda x: f"M{f(x+20)} 44a20 20 0 1 0 0.01 0M{f(x+40)} 18V84"),
    "z": (34, lambda x: f"M{f(x)} 44h34L{f(x)} 84h34"),
    "u": (36, lambda x: f"M{f(x)} 44V66a18 18 0 0 0 36 0M{f(x+36)} 44V84"),
    "k": (32, lambda x: f"M{f(x)} 18V84M{f(x+31)} 44L{f(x)} 70M{f(x+11)} 61L{f(x+32)} 84"),
    "e": (40, lambda x: f"M{f(x)} 64H{f(x+40)}A20 20 0 1 0 {f(x+34.1)} 78.1"),
    "m": (52, lambda x: f"M{f(x)} 44V84M{f(x)} 57a13 13 0 0 1 26 0V84M{f(x+26)} 57a13 13 0 0 1 26 0V84"),
    "t": (23, lambda x: f"M{f(x+7)} 26V73a11 11 0 0 0 11 11h4M{f(x)} 44H{f(x+23)}"),
    "r": (24, lambda x: f"M{f(x)} 44V84M{f(x)} 64a20 20 0 0 1 20 -20h4"),
    "i": (0, lambda x: f"M{f(x)} 44V84M{f(x)} 24h0.01"),
    "n": (36, lambda x: f"M{f(x)} 44V84M{f(x)} 62a18 18 0 0 1 36 0V84"),
    "v": (34, lambda x: f"M{f(x)} 44L{f(x+17)} 84L{f(x+34)} 44"),
    "-": (16, lambda x: f"M{f(x)} 64h16"),
    "p": (40, lambda x: f"M{f(x)} 44V106M{f(x+20)} 44a20 20 0 1 0 0.01 0"),
    "j": (0, lambda x: f"M{f(x)} 44V95a11 11 0 0 1 -11 11h-4M{f(x)} 24h0.01"),
    "y": (36, lambda x: f"M{f(x)} 44V66a18 18 0 0 0 36 0M{f(x+36)} 44V95a11 11 0 0 1 -11 11h-4"),
}


def wordmark(name, start=112):
    """Return (path data, x of each glyph, right edge of the last centre line)."""
    x, parts, xs = start, [], []
    for ch in name:
        w, d = GLYPHS[ch]
        parts.append(d(x))
        xs.append(x)
        x += w + GAP
    return "".join(parts), xs, x - GAP


def width_for(right):
    return int(right + STROKE / 2 + 3.5)  # ochazuke 509.1 -> 518, matomeru 524 -> 533


def word(d, ink):
    return (f'<path d="{d}" fill="none" stroke="{ink}" stroke-width="{STROKE}" '
            f'stroke-linecap="round" stroke-linejoin="round"/>\n')


def head(w, h, title):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" width="{w}" height="{h}" '
            f'role="img" aria-label="{title}">\n<title>{title}</title>\n')


def preview(src, out):
    """Render src's icon at 16, 32 and 128 px (and blown up 8x and 4x, to see
    the pixels) and both logos, into out (needs rsvg-convert and magick)."""
    import subprocess
    out.mkdir(parents=True, exist_ok=True)
    pngs = []
    for size in (16, 32, 128):
        p = out / f"icon-{size}.png"
        subprocess.run(["rsvg-convert", "-w", str(size), "-h", str(size), "-o", p, src / "icon.svg"], check=True)
        pngs.append(p)
    for theme, bg in (("light", "white"), ("dark", "#0d1117")):
        subprocess.run(["rsvg-convert", "-w", "720", "-b", bg, "-o", out / f"logo-{theme}.png",
                        src / f"logo-{theme}.svg"], check=True)
    big = []
    for size, k in ((16, 8), (32, 4)):
        b = out / f"icon-{size}-x{k}.png"
        subprocess.run(["magick", out / f"icon-{size}.png", "-filter", "point", "-resize", f"{k*100}%", b], check=True)
        big.append(b)
    subprocess.run(["magick", "-background", "#f6f3ee", *map(str, pngs), *map(str, big), "-bordercolor", "#f6f3ee",
                    "-border", "12", "-gravity", "center", "+append", out / "icons.png"], check=True)
    subprocess.run(["magick", out / "logo-light.png", out / "logo-dark.png", "-append", out / "logos.png"], check=True)
    print(out / "icons.png")
    print(out / "logos.png")


def check():
    """The glyph table must redraw ochazuke's and matomeru's wordmarks exactly."""
    import re
    ok = True
    for fam in ("ochazuke", "matomeru"):
        p = pathlib.Path.home() / f"proj/{fam}/assets/logo-light.svg"
        if not p.exists():
            print(f"skip {fam}: {p} not found")
            continue
        t = p.read_text()
        want = re.findall(r'<path d="(M[^"]*)" fill="none" stroke="#[0-9a-f]+" stroke-width="11"', t)[-1]
        want_w = int(re.search(r'viewBox="0 0 (\d+) 96"', t).group(1))
        d, _, right = wordmark(fam)
        split = lambda s: re.sub(r"([A-Za-z])", r" \1 ", s).split()
        a, b = split(d), split(want)
        near = lambda u, v: u == v or (u[0] not in "MmAaHhVvLlZz" and abs(float(u) - float(v)) < 0.15)
        same = len(a) == len(b) and all(map(near, a, b)) and width_for(right) == want_w
        ok &= same
        print(f"{fam}: {'same' if same else 'DIFFERENT'} wordmark ({width_for(right)} wide)")
    return ok


def icon():
    return (head(128, 128, NAME)
            + f'<rect width="128" height="128" rx="28" fill="{TILE}"/>\n'
            + mark("dark") + "\n</svg>\n")


def logo(theme):
    """The mark is narrow, so the name starts the family's 24-unit gap after
    it rather than at ochazuke's x 112."""
    ink = TILE if theme == "light" else PALE
    d, _, right = wordmark(NAME, MARK_RIGHT + 24 + STROKE / 2)
    return (head(width_for(right), HEIGHT, NAME)
            + f'<g transform="{LOGO_MARK}">{mark(theme)}</g>\n' + word(d, ink) + "</svg>\n")


def main():
    if sys.argv[1:] == ["--check"]:
        sys.exit(0 if check() else 1)
    files = {"icon.svg": icon(), "logo-light.svg": logo("light"), "logo-dark.svg": logo("dark")}
    for name, text in files.items():
        (ASSETS / name).write_text(text)
        print(ASSETS / name)
    if len(sys.argv) == 3 and sys.argv[1] == "--preview":
        preview(ASSETS, pathlib.Path(sys.argv[2]))


if __name__ == "__main__":
    main()
