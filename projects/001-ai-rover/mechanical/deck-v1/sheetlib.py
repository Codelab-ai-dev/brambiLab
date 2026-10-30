"""Utilidades comunes de los planos: proyección ortográfica real del sólido
(TechDraw.projectEx) convertida a SVG propio, cotas y página TechDraw con el
SVG como símbolo. Se usa así porque el exportador PDF de TechDraw en 1.1.4
coloca mal las vistas de un DrawProjGroup creado sin interfaz."""
import math
import FreeCAD as App
from FreeCAD import Vector as V
import TechDraw

CSS = """<style>
.vis{fill:none;stroke:#000;stroke-width:0.35;stroke-linecap:round;stroke-linejoin:round}
.hid{fill:none;stroke:#555;stroke-width:0.18;stroke-dasharray:1.2 0.8}
.thin{fill:none;stroke:#000;stroke-width:0.18}
.chassis{fill:none;stroke:#444;stroke-width:0.25;stroke-dasharray:6 1.5 1 1.5}
.support{fill:#bbb;fill-opacity:0.45;stroke:#555;stroke-width:0.2}
.zone{fill:#2a9d4a;fill-opacity:0.07;stroke:#2a9d4a;stroke-width:0.3;stroke-dasharray:3 1.5}
.board{fill:#1f5fbf;fill-opacity:0.05;stroke:#1f5fbf;stroke-width:0.3;stroke-dasharray:3 1.5}
.dim{stroke:#000;stroke-width:0.15;fill:none}
.arr{fill:#000;stroke:none}
.grid{stroke:#999;stroke-width:0.12}
.frame{fill:none;stroke:#000;stroke-width:0.5}
text{font-family:Arial,Helvetica,sans-serif;fill:#000}
.z{fill:#333;font-style:italic}
.h{font-weight:bold}
</style>"""


def mat(rows):
    m = App.Matrix()
    (m.A11, m.A12, m.A13), (m.A21, m.A22, m.A23), (m.A31, m.A32, m.A33) = rows
    return m


def _n(v):
    l = math.sqrt(sum(c * c for c in v)); return [c / l for c in v]


def _cross(a, b):
    return [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]]


def iso_rows(viewer=(1, -1, 1)):
    n = _n(viewer); r = _n(_cross([0, 0, 1], n)); u = _cross(n, r)
    return [r, u, n]


# filas = ejes de pantalla (derecha, arriba, hacia el observador) en coordenadas del modelo
VIEWS = {
    "top": [[1, 0, 0], [0, 1, 0], [0, 0, 1]],
    "bottom": [[1, 0, 0], [0, -1, 0], [0, 0, -1]],
    "front": [[1, 0, 0], [0, 0, 1], [0, -1, 0]],     # observador en -Y
    "right": [[0, 1, 0], [0, 0, 1], [1, 0, 0]],      # observador en +X
    "left": [[0, -1, 0], [0, 0, 1], [-1, 0, 0]],
    "iso": iso_rows(),
    "side_zy": [[0, 0, 1], [0, 1, 0], [-1, 0, 0]],   # z a la derecha, y arriba
}


def pts_of_edge(e, prev=None):
    d = e.discretize(Deflection=0.02)
    if prev is not None and (d[-1] - prev).Length < (d[0] - prev).Length:
        d = d[::-1]
    return d


class View:
    """Proyección de una forma colocada en la hoja: escala s, esquina (x0, y0) del
    rectángulo destino (y hacia abajo, en mm de hoja)."""

    def __init__(self, shape, view, s, cx, cy, hidden=True):
        self.rows = VIEWS[view]
        sh = shape.copy()
        sh.transformShape(mat(self.rows))
        self.res = TechDraw.projectEx(sh, V(0, 0, 1))
        bb = self.res[0].BoundBox
        for c in self.res[1:5]:
            if c.Edges:
                bb.add(c.BoundBox)
        self.bb, self.s = bb, s
        # centro de la vista en (cx, cy)
        self.ox = cx - s * (bb.XMin + bb.XMax) / 2
        self.oy = cy + s * (bb.YMin + bb.YMax) / 2
        self.hidden = hidden
        self.w, self.h = s * bb.XLength, s * bb.YLength

    def P(self, x, y):
        return self.ox + self.s * x, self.oy - self.s * y

    def M(self, p):
        """Punto del modelo -> hoja."""
        r = self.rows
        return self.P(sum(a * b for a, b in zip(r[0], p)), sum(a * b for a, b in zip(r[1], p)))

    def svg(self):
        out = []
        groups = [(0, "vis"), (1, "vis"), (3, "vis")] + ([(5, "hid")] if self.hidden else [])
        for idx, cls in groups:
            comp = self.res[idx]
            for e in comp.Edges:
                d = pts_of_edge(e)
                s = "M %.3f %.3f " % self.P(d[0].x, d[0].y) + " ".join(
                    "L %.3f %.3f" % self.P(p.x, p.y) for p in d[1:])
                out.append('<path d="%s" class="%s"/>' % (s, cls))
        return out

    def top_y(self):
        return self.oy - self.s * self.bb.YMax

    def bottom_y(self):
        return self.oy - self.s * self.bb.YMin

    def left_x(self):
        return self.ox + self.s * self.bb.XMin

    def right_x(self):
        return self.ox + self.s * self.bb.XMax


def txt(x, y, s, size=2.5, anchor="middle", rot=0, cls="t"):
    s = str(s).replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    tr = ' transform="rotate(%g %.3f %.3f)"' % (rot, x, y) if rot else ""
    return '<text x="%.3f" y="%.3f" font-size="%g" text-anchor="%s" class="%s"%s>%s</text>' % (
        x, y, size, anchor, cls, tr, s)


def _arrow(x, y, ang, L=2.2, W=0.7):
    c, s = math.cos(ang), math.sin(ang)
    p1 = (x - L * c + W * s, y - L * s - W * c)
    p2 = (x - L * c - W * s, y - L * s + W * c)
    return '<path d="M %.3f %.3f L %.3f %.3f L %.3f %.3f Z" class="arr"/>' % (x, y, *p1, *p2)


def dim(a, b, off, value, horizontal=True, size=2.5, label=None):
    """Cota lineal entre los puntos de hoja a y b, desplazada `off` mm
    (horizontal: línea a y=min(a,b)+off; vertical: x=min+off)."""
    g = []
    lab = label or ("%g" % round(value, 2))
    if horizontal:
        y = off
        g.append('<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" class="dim"/>' % (a[0], a[1], a[0], y))
        g.append('<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" class="dim"/>' % (b[0], b[1], b[0], y))
        x1, x2 = sorted((a[0], b[0]))
        g.append('<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" class="dim"/>' % (x1, y, x2, y))
        g.append(_arrow(x1, y, math.pi)); g.append(_arrow(x2, y, 0))
        if x2 - x1 < 8:
            g.append(txt(x2 + 1.5, y - 0.8, lab, size, "start"))
        else:
            g.append(txt((x1 + x2) / 2, y - 0.8, lab, size))
    else:
        x = off
        g.append('<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" class="dim"/>' % (a[0], a[1], x, a[1]))
        g.append('<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" class="dim"/>' % (b[0], b[1], x, b[1]))
        y1, y2 = sorted((a[1], b[1]))
        g.append('<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" class="dim"/>' % (x, y1, x, y2))
        g.append(_arrow(x, y1, -math.pi / 2)); g.append(_arrow(x, y2, math.pi / 2))
        if y2 - y1 < 8:
            g.append(txt(x - 0.8, y1 - 1.5, lab, size, rot=-90, anchor="start"))
        else:
            g.append(txt(x - 0.8, (y1 + y2) / 2, lab, size, rot=-90))
    return g


def title_block(W, H, title, number, scale, sheet, notes=(), notes_xy=None, date="2026-10-13"):
    """Marco y cajetín propios (ISO 5457 simplificado)."""
    g = ['<rect x="10" y="10" width="%g" height="%g" class="frame"/>' % (W - 20, H - 20)]
    x0, y0, bw, bh = W - 190, H - 36, 180, 26
    g.append('<rect x="%g" y="%g" width="%g" height="%g" class="frame"/>' % (x0, y0, bw, bh))
    for yy in (y0 + 9, y0 + 17.5):
        g.append('<line x1="%g" y1="%g" x2="%g" y2="%g" class="thin"/>' % (x0, yy, x0 + bw, yy))
    for xx in (x0 + 100, x0 + 140):
        g.append('<line x1="%g" y1="%g" x2="%g" y2="%g" class="thin"/>' % (xx, y0 + 9, xx, y0 + bh))
    g.append(txt(x0 + 2, y0 + 6.3, title, 4.2, "start", cls="h"))
    g.append(txt(x0 + 2, y0 + 14.5, "brambiLab · Rover 4WD (001-ai-rover)", 2.8, "start"))
    g.append(txt(x0 + 2, y0 + 23, "Material PLA · mm · ISO 2768-m · 1er diedro", 2.8, "start"))
    g.append(txt(x0 + 102, y0 + 14.5, "Nº " + number, 2.8, "start"))
    g.append(txt(x0 + 102, y0 + 23, "Escala " + scale, 2.8, "start"))
    g.append(txt(x0 + 142, y0 + 14.5, "Hoja " + sheet, 2.8, "start"))
    g.append(txt(x0 + 142, y0 + 23, "v1 · " + date, 2.8, "start"))
    for i, n in enumerate(notes):
        if notes_xy:
            g.append(txt(notes_xy[0], notes_xy[1] + 4.6 * i, n, 2.5, "start"))
        else:
            g.append(txt(x0, y0 - 4 - 4.6 * (len(notes) - 1 - i), n, 2.5, "start"))
    return g


def write_sheet(path_base, W, H, elements, page_name):
    svg = ('<svg xmlns="http://www.w3.org/2000/svg" width="%gmm" height="%gmm" viewBox="0 0 %g %g">'
           '<rect x="0" y="0" width="%g" height="%g" fill="#fff"/>%s\n%s</svg>'
           % (W, H, W, H, W, H, CSS, "\n".join(elements)))
    with open(path_base + ".svg", "w") as fh:
        fh.write(svg)
    d = App.newDocument(page_name)
    App.setActiveDocument(d.Name)
    page = d.addObject("TechDraw::DrawPage", page_name)
    t = d.addObject("TechDraw::DrawSVGTemplate", "Plantilla")
    t.Template = App.getResourceDir() + "Mod/TechDraw/Templates/ISO/%s_Landscape_blank.svg" % (
        "A3" if W > 400 else "A4")
    page.Template = t
    sym = d.addObject("TechDraw::DrawViewSymbol", "Hoja")
    sym.Symbol = svg
    page.addView(sym)
    sym.Scale = 1.0
    sym.X, sym.Y = W / 2, H / 2
    d.recompute()
    d.saveAs(path_base + ".FCStd")
    App.closeDocument(d.Name)
    return path_base
