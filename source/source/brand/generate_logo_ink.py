"""Ink-style logo inspired by the Ligue corse emblem (without the Moor's head):
white king standing, black king and black queen lying at his feet."""
import importlib.util
spec = importlib.util.spec_from_file_location("gen", "/home/claude/logo/gen.py")
src = open("/home/claude/logo/gen.py").read().split("K = mirror_path")[0]
ns = {}
exec(src, ns)
K = ns["mirror_path"](ns["king"], -296)
Q = ns["mirror_path"](ns["queen"], -212)
svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
  <title>pairing_easy</title>
  <defs>
    <path id="king" d="{K}"/>
    <path id="queen" d="{Q}"/>
    <g id="queenFull"><use href="#queen"/>
      <circle cx="-56" cy="-238" r="10"/><circle cx="56" cy="-238" r="10"/><circle cx="-16" cy="-250" r="9"/><circle cx="16" cy="-250" r="9"/><circle cx="0" cy="-222" r="9"/></g>
  </defs>
  <rect width="512" height="512" fill="#fff"/>
  <!-- sol : trait d'encre -->
  <path d="M28 496 C150 488 362 488 484 496" fill="none" stroke="#111" stroke-width="7" stroke-linecap="round"/>
  <g stroke="#111" stroke-width="3" stroke-linecap="round" opacity=".8">
    
  </g>

  <!-- roi blanc debout : contour à l'encre et hachures d'ombre -->
  <g transform="translate(256 404) scale(1.12)">
    <clipPath id="kclip"><use href="#king"/></clipPath>
    <use href="#king" fill="#fff"/>
    <g clip-path="url(#kclip)" stroke="#111" stroke-width="3.2" stroke-linecap="round">
      <path d="M16 -60 L30 -74"/><path d="M14 -80 L30 -96"/><path d="M14 -100 L30 -116"/><path d="M14 -120 L30 -136"/>
      <path d="M16 -140 L30 -154"/><path d="M18 -158 L32 -172"/><path d="M28 -206 L46 -224"/><path d="M22 -220 L40 -238"/>
      <path d="M40 -36 L62 -48"/><path d="M50 -10 L74 -22"/>
    </g>
    <use href="#king" fill="none" stroke="#111" stroke-width="7" stroke-linejoin="round"/>
    <path d="M-50 -178 L50 -178 M-60 -29 L60 -29" stroke="#111" stroke-width="5" stroke-linecap="round"/>
  </g>
  <!-- roi noir couché (à gauche), reflets blancs gravés -->
  <g transform="translate(236 452) rotate(-84) scale(.64)">
    <use href="#king" fill="#111"/>
    <g fill="none" stroke="#fff" stroke-width="5" stroke-linecap="round">
      <path d="M18 -70 C14 -110 12 -140 16 -168"/><path d="M22 -200 C34 -210 36 -228 24 -236"/><path d="M40 -14 L62 -14"/>
    </g>
  </g>
  <!-- dame noire couchée (à droite) -->
  <g transform="translate(278 456) rotate(86) scale(.66)">
    <use href="#queenFull" fill="#111"/>
    <g fill="none" stroke="#fff" stroke-width="5" stroke-linecap="round">
      <path d="M-16 -66 C-12 -104 -10 -132 -14 -156"/><path d="M-38 -12 L-60 -12"/>
    </g>
  </g>

</svg>
'''
open("logo-ink.svg", "w").write(svg)
