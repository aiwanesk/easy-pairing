"""Logo inspired by the Ligue corse emblem (with its president's agreement):
the Moor's head is removed; the toppled piece becomes black (ink negative),
the white king stays standing. Output: brand/logo-corse-*.png"""
from PIL import Image, ImageFilter
import numpy as np
from scipy import ndimage

SRC = '/mnt/user-data/uploads/afd4123f-image.png'
im = np.array(Image.open(SRC).convert('L'))
inner = np.zeros(im.shape, bool); inner[22:345, 28:340] = True
blk = im < 128

# 1. remove the head
lab, n = ndimage.label(blk & inner)
sizes = ndimage.sum(np.ones_like(lab), lab, range(1, n + 1))
head_id = int(np.argmax(sizes)) + 1
head = lab == head_id
dil = ndimage.binary_dilation(head, iterations=5)
for i in range(1, n + 1):
    if i != head_id:
        c = lab == i
        if c.sum() < 250 and not (c & ~dil).any():
            head |= c
wlab, wn = ndimage.label((~blk) & inner)
out = im.copy()
out[head] = 255
contour = np.zeros(im.shape, bool)
# contour of the standing pieces where the head outlined them
near = ndimage.binary_dilation(head, iterations=3) & (~blk) & inner
for wid in np.unique(wlab[near]):
    if wid == 0: continue
    comp = wlab == wid
    ys, xs = np.where(comp)
    if xs.min() >= 200 and comp.sum() >= 300:
        e = head & ndimage.binary_dilation(comp, iterations=3)
        out[e] = 0
        contour |= e
# remove every trace of the head (anti-aliasing specks) except the other drawings
keep = np.zeros(im.shape, bool)
for i in range(1, n + 1):
    if i != head_id and sizes[i - 1] >= 150:
        keep |= lab == i
zone = ndimage.binary_dilation(head, iterations=9) & inner
out[zone & ~ndimage.binary_dilation(keep, iterations=2) & ~contour] = 255

# 2. toppled piece -> black: white areas become ink, inner strokes become highlights
lying = np.zeros(im.shape, bool)
for wid in range(1, wn + 1):
    comp = wlab == wid
    s = comp.sum()
    if s < 15: continue
    ys, xs = np.where(comp)
    if ys.min() > 200 and (xs.max() < 205 or (xs.min() >= 190 and xs.max() <= 252 and ys.max() > 330)):
        lying |= comp
shape = ndimage.binary_fill_holes(ndimage.binary_dilation(lying, iterations=3))
interior = ndimage.binary_erosion(shape, iterations=3)
neg = out.copy()
neg[interior] = 255 - out[interior]
neg[shape & ~interior] = np.minimum(out[shape & ~interior], 40)
out = neg

# 3. square crop around the drawing, white background
o = out[25:343, 31:338]
ys, xs = np.where(o < 128)
x0, x1, y0, y1 = xs.min(), xs.max(), ys.min(), ys.max()
side = max(x1 - x0, y1 - y0) + 36
cx, cy = (x0 + x1) // 2, (y0 + y1) // 2
pad = np.full((o.shape[0] + 2 * side, o.shape[1] + 2 * side), 255, np.uint8)
pad[side:side + o.shape[0], side:side + o.shape[1]] = o
sq = pad[cy + side - side // 2: cy + side - side // 2 + side, cx + side - side // 2: cx + side - side // 2 + side]
img = Image.fromarray(sq)
img.resize((1024, 1024), Image.LANCZOS).save('/home/claude/logo/logo-corse-1024.png')
img.resize((512, 512), Image.LANCZOS).save('/home/claude/logo/logo-corse-512.png')
img.resize((64, 64), Image.LANCZOS).save('/home/claude/logo/logo-corse-64.png')
print(side)
