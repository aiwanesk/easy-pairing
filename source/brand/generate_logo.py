"""Generate the pairing_easy logo: standing white king, toppled black king and queen."""
def mirror_path(segs, top):
    """segs: right-half segments from (0,0) bottom centre to (0,top), each ('L',(x,y)) or ('C',c1,c2,(x,y)).
    Returns a closed symmetric path string."""
    pts = [(0, 0)]
    for s in segs:
        pts.append(s[-1])
    d = "M0 0 "
    for s in segs:
        if s[0] == 'L':
            d += f"L{s[1][0]} {s[1][1]} "
        else:
            d += f"C{s[1][0]} {s[1][1]} {s[2][0]} {s[2][1]} {s[3][0]} {s[3][1]} "
    # left half: traverse backwards, mirrored
    for i in range(len(segs) - 1, -1, -1):
        s = segs[i]
        start = pts[i]
        if s[0] == 'L':
            d += f"L{-start[0]} {start[1]} "
        else:
            d += f"C{-s[2][0]} {s[2][1]} {-s[1][0]} {s[1][1]} {-start[0]} {start[1]} "
    return d + "Z"

king = [
    ('L', (70, 0)), ('C', (78, 0), (80, -6), (80, -12)), ('L', (80, -18)), ('C', (80, -26), (74, -28), (66, -29)),
    ('C', (60, -30), (58, -36), (56, -42)), ('C', (50, -54), (40, -58), (36, -62)),
    ('C', (30, -110), (26, -150), (30, -176)),
    ('L', (50, -178)), ('C', (58, -178), (60, -186), (54, -190)), ('L', (40, -194)),
    ('C', (56, -206), (56, -234), (34, -244)), ('L', (9, -248)),
    ('L', (9, -262)), ('L', (24, -262)), ('L', (24, -276)), ('L', (9, -276)), ('L', (9, -296)), ('L', (0, -296)),
]
queen = [
    ('L', (66, 0)), ('C', (74, 0), (76, -6), (76, -11)), ('L', (76, -17)), ('C', (76, -24), (70, -26), (62, -27)),
    ('C', (56, -28), (54, -34), (52, -39)), ('C', (46, -50), (37, -54), (33, -58)),
    ('C', (27, -104), (24, -140), (28, -164)),
    ('L', (46, -166)), ('C', (54, -166), (56, -174), (50, -178)), ('L', (38, -181)),
    ('L', (56, -232)), ('L', (30, -206)), ('L', (16, -244)), ('L', (0, -212)),
]
K = mirror_path(king, -296)
Q = mirror_path(queen, -212)
svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
  <title>pairing_easy</title>
  <defs>
    <path id="king" d="{K}"/>
    <path id="queen" d="{Q}"/>
    <clipPath id="right"><rect x="0" y="-320" width="120" height="340"/></clipPath>
  </defs>
  <rect width="512" height="512" rx="116" fill="#2a7a62"/>
  <ellipse cx="256" cy="430" rx="196" ry="20" fill="#1f5c4a"/>
  <!-- black king and queen, toppled -->
  <g fill="#0d1f1a">
    <g transform="translate(176 424) rotate(-90) scale(.52)"><use href="#king"/></g>
    <g transform="translate(338 426) rotate(90) scale(.56)"><use href="#queen"/>
      <circle cx="-56" cy="-238" r="10"/><circle cx="56" cy="-238" r="10"/><circle cx="-16" cy="-250" r="9"/><circle cx="16" cy="-250" r="9"/><circle cx="0" cy="-222" r="9"/></g>
  </g>
  <!-- white king, standing -->
  <g transform="translate(256 430) scale(1.12)">
    <use href="#king" fill="#ffffff"/>
    <use href="#king" fill="#d8e5dd" clip-path="url(#right)"/>
  </g>
</svg>
'''
open("logo.svg", "w").write(svg)
