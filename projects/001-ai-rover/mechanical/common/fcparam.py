"""Utilidades paramétricas compartidas por los modelos FreeCAD del rover.

E          cota con valor numérico y expresión FreeCAD (se combinan con + - * /)
build_params(doc, sheet_name, PARAMS)  hoja de cálculo con alias y fórmulas
Sk         sketch totalmente restringido, atado a la hoja por expresiones
feat       feature PartDesign con propiedades atadas a expresiones
try_fillet fillet o chaflán PartDesign seleccionando aristas por geometría
Copiado de deck-v1/build_deck.py, que conserva su propia copia.
"""
import math
import FreeCAD as App
from FreeCAD import Vector as V
import Part
import Sketcher

class E:
    def __init__(s, v, x, unit=True):
        s.v, s.x, s.u = float(v), x, unit

    @staticmethod
    def k(v, unit=True):
        return E(v, ("(%r mm)" % float(v)) if unit else repr(float(v)), unit)

    def _w(s, o):
        return o if isinstance(o, E) else E.k(o, s.u)

    def __add__(s, o):
        o = s._w(o); return E(s.v + o.v, "(%s + %s)" % (s.x, o.x), s.u)
    __radd__ = __add__

    def __sub__(s, o):
        o = s._w(o); return E(s.v - o.v, "(%s - %s)" % (s.x, o.x), s.u)

    def __rsub__(s, o):
        o = s._w(o); return E(o.v - s.v, "(%s - %s)" % (o.x, s.x), s.u)

    def __mul__(s, o):
        if isinstance(o, E):
            return E(s.v * o.v, "(%s * %s)" % (s.x, o.x), s.u or o.u)
        return E(s.v * o, "(%s * %r)" % (s.x, float(o)), s.u)
    __rmul__ = __mul__

    def __truediv__(s, o):
        return E(s.v / o, "(%s / %r)" % (s.x, float(o)), s.u)

    def __neg__(s):
        return E(-s.v, "(-%s)" % s.x, s.u)



def build_params(doc, sheet_name, PARAMS):
    SHEET[0] = sheet_name
    sh = doc.addObject("Spreadsheet::Sheet", sheet_name)
    P, vals, row = {}, {}, 1
    sh.set("A1", "parámetro"); sh.set("B1", "valor"); sh.set("C1", "nota")
    for item in PARAMS:
        row += 1
        if len(item) == 1:
            sh.set("A%d" % row, item[0]); continue
        name, val, unit, formula, note = item
        cell = "B%d" % row
        sh.set("A%d" % row, name)
        if formula:
            py = formula[1:].replace(" mm", "").replace(" deg", "").replace("^", "**")
            val = eval(py, {"sqrt": math.sqrt, "sin": lambda d: math.sin(math.radians(d)),
                            "cos": lambda d: math.cos(math.radians(d))}, vals)
            sh.set(cell, formula)
        else:
            txt = ("%r %s" % (float(val), unit)).strip() if unit else repr(float(val))
            sh.set(cell, ("=" + txt) if float(val) < 0 else txt)   # "-2 mm" sin "=" se guarda como texto
        sh.setAlias(cell, name)
        sh.set("C%d" % row, ("'" + note) if note[:1] in "=+-" and note else note)
        vals[name] = float(val)
        P[name] = E(val, "%s.%s" % (sheet_name, name), unit == "mm")
    doc.recompute()
    return P, vals


class Sk:
    def __init__(s, body, name, plane="XY_Plane", offset=None):
        s.body = body
        s.o = body.newObject("Sketcher::SketchObject", name)
        pl = [f for f in body.Origin.OriginFeatures if f.Role == plane][0]
        s.o.AttachmentSupport = [(pl, "")]
        s.o.MapMode = "FlatFace"
        s.binds = []
        if offset is not None:
            s.o.AttachmentOffset = App.Placement(V(0, 0, offset.v), App.Rotation())
            if SHEET[0] in offset.x:
                s.o.setExpression(".AttachmentOffset.Base.z", offset.x)

    def _c(s, c, e=None):
        i = s.o.addConstraint(c)
        if e is not None and SHEET[0] in e.x:
            s.binds.append((i, e.x))
        return i

    def _fix_point(s, g, pos, x, y):
        s._c(Sketcher.Constraint("DistanceX", -1, 1, g, pos, x.v), x)
        s._c(Sketcher.Constraint("DistanceY", -1, 1, g, pos, y.v), y)

    def circle(s, cx, cy, r):
        g = s.o.addGeometry(Part.Circle(V(cx.v, cy.v, 0), V(0, 0, 1), r.v), False)
        s._fix_point(g, 3, cx, cy)
        s._c(Sketcher.Constraint("Radius", g, r.v), r)
        return g

    def poly(s, pts, radii=None):
        """Polígono CCW; radii[i] (E o None) redondea el vértice i (sólo
        esquinas convexas y ejes alineados)."""
        n = len(pts)
        radii = radii or [None] * n
        # dirección de cada lado i: pts[i] -> pts[i+1]
        dirs = []
        for i in range(n):
            a, b = pts[i], pts[(i + 1) % n]
            dx, dy = b[0].v - a[0].v, b[1].v - a[1].v
            L = math.hypot(dx, dy)
            dirs.append((dx / L, dy / L))
        # extremos de cada lado, recortados por los redondeos
        starts, ends = [], []
        for i in range(n):
            j = (i + 1) % n
            sx, sy = pts[i]
            ex, ey = pts[j]
            if radii[i] is not None:
                sx = sx + radii[i] * dirs[i][0]; sy = sy + radii[i] * dirs[i][1]
            if radii[j] is not None:
                ex = ex - radii[j] * dirs[i][0]; ey = ey - radii[j] * dirs[i][1]
            starts.append((sx, sy)); ends.append((ex, ey))
        lines = []
        for i in range(n):
            (sx, sy), (ex, ey) = starts[i], ends[i]
            lines.append(s.o.addGeometry(Part.LineSegment(V(sx.v, sy.v, 0), V(ex.v, ey.v, 0)), False))
        for j in range(n):          # vértice j: entre el lado j-1 y el lado j
            i = (j - 1) % n
            if radii[j] is None:
                s._c(Sketcher.Constraint("Coincident", lines[i], 2, lines[j], 1))
                s._fix_point(lines[j], 1, pts[j][0], pts[j][1])
            else:
                r = radii[j]
                din, dout = dirs[i], dirs[j]
                assert din[0] * dout[1] - din[1] * dout[0] > 0.5, "redondeo en esquina cóncava"
                cx = pts[j][0] - r * din[0] + r * dout[0]
                cy = pts[j][1] - r * din[1] + r * dout[1]
                a0 = math.atan2(ends[i][1].v - cy.v, ends[i][0].v - cx.v)
                a1 = math.atan2(starts[j][1].v - cy.v, starts[j][0].v - cx.v)
                while a1 <= a0:
                    a1 += 2 * math.pi
                arc = s.o.addGeometry(Part.ArcOfCircle(
                    Part.Circle(V(cx.v, cy.v, 0), V(0, 0, 1), r.v), a0, a1), False)
                s._fix_point(arc, 3, cx, cy)
                s._c(Sketcher.Constraint("Radius", arc, r.v), r)
                s._c(Sketcher.Constraint("Tangent", lines[i], 2, arc, 1))
                s._c(Sketcher.Constraint("Tangent", arc, 2, lines[j], 1))
        return lines

    def rect(s, x0, y0, x1, y1, r=None):
        return s.poly([(x0, y0), (x1, y0), (x1, y1), (x0, y1)], [r] * 4)

    def rect_c(s, cx, cy, lx, ly, r=None):
        return s.rect(cx - lx / 2, cy - ly / 2, cx + lx / 2, cy + ly / 2, r)

    def hexagon(s, cx, cy, af):
        R = af / math.sqrt(3)       # radio circunscrito
        pts = [(cx + R * math.cos(math.radians(30 + 60 * k)),
                cy + R * math.sin(math.radians(30 + 60 * k))) for k in range(6)]
        return s.poly(pts)

    def finish(s):
        for i, x in s.binds:
            s.o.setExpression(".Constraints[%d]" % i, x)
        return s.o


SKETCHES = []
SHEET = ["Params"]   # build_params lo actualiza con el nombre de la hoja


def sk(body, name, plane="XY_Plane", offset=None):
    o = Sk(body, name, plane, offset)
    SKETCHES.append(o)
    return o


def feat(body, kind, name, profile, **props):
    f = body.newObject("PartDesign::" + kind, name)
    f.Profile = profile.finish() if isinstance(profile, Sk) else profile
    for k, v in props.items():
        if isinstance(v, E):
            setattr(f, k, v.v)
            if SHEET[0] in v.x:
                f.setExpression(k, v.x)
        else:
            setattr(f, k, v)
    return f



def try_fillet(doc, body, name, pred, r, kind="Fillet"):
    tip = body.Tip
    edges = ["Edge%d" % (i + 1) for i, e in enumerate(tip.Shape.Edges) if pred(e)]
    if not edges:
        return None, 0
    f = body.newObject("PartDesign::" + kind, name)
    f.Base = (tip, edges)
    if kind == "Fillet":
        f.Radius = r
    else:
        f.Size = r
    doc.recompute()
    if "Invalid" in f.State or "Error" in f.State or not f.Shape.isValid():
        body.removeObject(f); doc.removeObject(f.Name); doc.recompute()
        return None, len(edges)
    return f, len(edges)


