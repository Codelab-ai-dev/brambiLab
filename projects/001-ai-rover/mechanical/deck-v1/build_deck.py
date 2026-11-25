"""Segundo piso modular del rover 4WD (deck v1) — modelo paramétrico FreeCAD.

Genera un .FCStd con:
  Params      hoja de cálculo con todos los parámetros (editar aquí)
  Deck        cuerpo PartDesign del piso superior (sketches 100 % restringidos)
  Support     cuerpo PartDesign de un soporte (se imprime 4 veces)
  Support_*   App::Link de los 4 soportes, colocados por expresión
  ChassisRef  placa inferior de referencia con el patrón de orificios cuadrados
  UnoQ_Env    envolvente de referencia del Arduino UNO Q (no se imprime)

Ejecutar:
  ~/.claude/skills/freecad/scripts/fc.sh run build_deck.py -- --out <carpeta>

Sistema de coordenadas: origen en la esquina de la placa del chasis, X a lo
largo (X=0 es el FRENTE supuesto), Y a lo ancho, Z=0 cara superior del chasis.
"""
import sys, os, math, json
sys.path.insert(0, os.path.expanduser("~/.claude/skills/freecad/scripts"))
from fchelpers import *  # noqa: F401,F403
import Sketcher
import Mesh, MeshPart, Import


# --------------------------------------------------------------------------- #
# Expresiones: cada cota lleva su valor numérico y su expresión FreeCAD
# --------------------------------------------------------------------------- #
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


# --------------------------------------------------------------------------- #
# Parámetros: (nombre, valor, unidad, fórmula o None, nota)
# --------------------------------------------------------------------------- #
PARAMS = [
    ("# Chasis existente (medido)",),
    ("chassis_length", 160, "mm", None, "placa inferior de acrílico"),
    ("chassis_width", 90, "mm", None, ""),
    ("chassis_thickness", 3, "mm", None, "SUPUESTO: medir el acrílico"),
    ("square_hole_size", 2.89, "mm", None, "orificio cuadrado medido"),
    ("square_hole_pitch", 6.97, "mm", None, "centro a centro"),
    ("holes_per_group", 4, "", None, ""),
    ("side_offset", 4.41, "mm", None, "centro de fila al borde lateral"),
    ("row_1_y", None, "mm", "=side_offset", ""),
    ("row_2_y", None, "mm", "=chassis_width - side_offset", ""),
    ("row_spacing", None, "mm", "=row_2_y - row_1_y", "comprobación: 81.18"),
    ("group_1_x", 26, "mm", None, "centro del 1er orificio del grupo 1 (CONFIRMAR referencia)"),
    ("group_spacing_x", 50, "mm", None, ""),
    ("group_2_x", None, "mm", "=group_1_x + group_spacing_x", ""),
    ("# Piso superior",),
    ("deck_length", 145, "mm", None, ""),
    ("deck_width", 80, "mm", None, ""),
    ("deck_thickness", 3, "mm", None, "15 capas de 0.2"),
    ("deck_height", 40, "mm", None, "chasis -> cara inferior de la placa; rango 35-45"),
    ("deck_x0", None, "mm", "=(chassis_length - deck_length) / 2", "centrado en el chasis"),
    ("deck_y0", None, "mm", "=(chassis_width - deck_width) / 2", ""),
    ("deck_corner_r", 6, "mm", None, ""),
    ("tab_corner_r", 1.5, "mm", None, "esquinas de las orejetas"),
    ("rib_thickness", 2.4, "mm", None, "6 líneas de 0.4"),
    ("rib_height", 6, "mm", None, "bajo la placa"),
    ("# Retícula de nervaduras (líneas centrales)",),
    ("rib_x_1", 19, "mm", None, "fin zona frontal"),
    ("rib_x_2", 45, "mm", None, ""),
    ("rib_x_3", 71, "mm", None, "frontera LiDAR / UNO Q"),
    ("rib_x_4", 88.5, "mm", None, ""),
    ("rib_x_5", 106, "mm", None, ""),
    ("rib_x_6", 123.5, "mm", None, ""),
    ("rib_x_7", 141, "mm", None, "inicio franja trasera"),
    ("rib_y_1", 18, "mm", None, "frontera zona potencia"),
    ("rib_y_2", 32, "mm", None, ""),
    ("rib_y_3", None, "mm", "=chassis_width / 2", ""),
    ("rib_y_4", None, "mm", "=chassis_width - rib_y_2", ""),
    ("rib_y_5", None, "mm", "=chassis_width - rib_y_1", "frontera zona señales"),
    ("# Soportes (4 piezas iguales)",),
    ("support_hole_index", 1, "", None, "usa orificios index e index+1 del grupo (0..2)"),
    ("locator_size", 2.55, "mm", None, "guía cuadrada; holgura 0.34 en 2.89"),
    ("locator_height", 2.5, "mm", None, "< espesor del acrílico"),
    ("fastener_nominal", 2.5, "mm", None, "M2.5"),
    ("m25_clearance", 2.9, "mm", None, "pasante M2.5"),
    ("m25_pilot", 2.2, "mm", None, "roscar M2.5 en PLA"),
    ("pilot_depth", 10, "mm", None, ""),
    ("col_length", 16, "mm", None, "columna, a lo largo de X"),
    ("col_out", 3.5, "mm", None, "columna hacia el borde desde la fila"),
    ("col_in", 4.5, "mm", None, "columna hacia el centro desde la fila"),
    ("col_corner_r", 2, "mm", None, ""),
    ("foot_length", 24, "mm", None, ""),
    ("foot_in", 9.5, "mm", None, "pie hacia el centro desde la fila"),
    ("foot_thickness", 3, "mm", None, ""),
    ("foot_transition", 5, "mm", None, "transición a 45° pie -> columna"),
    ("gusset_height", 24, "mm", None, "nervaduras del soporte"),
    ("gusset_thickness", None, "mm", "=rib_thickness", ""),
    ("gusset_pitch", 5, "mm", None, "nervaduras laterales a +-pitch"),
    ("socket_clearance", 0.15, "mm", None, "por lado, columna en su alojamiento"),
    ("# Arduino UNO Q (VALORES PROVISIONALES: medir la placa)",),
    ("unoq_length", 68.6, "mm", None, "PROVISIONAL forma UNO"),
    ("unoq_width", 53.4, "mm", None, "PROVISIONAL forma UNO"),
    ("unoq_hole_spacing_x", 58.6, "mm", None, "PROVISIONAL, no es el patrón real"),
    ("unoq_hole_spacing_y", 43.4, "mm", None, "PROVISIONAL, no es el patrón real"),
    ("unoq_mount_offset_x", 106, "mm", None, "X del centro de la placa"),
    ("unoq_mount_offset_y", None, "mm", "=chassis_width / 2", "Y del centro de la placa"),
    ("standoff_diameter", 6, "mm", None, "separador comercial M2.5 (no impreso), referencia"),
    ("standoff_height", 6, "mm", None, "separador comercial M2.5 hembra-macho; altura de la placa UNO Q"),
    ("unoq_boss_d", 7.4, "mm", None, "refuerzo bajo cada separador"),
    ("unoq_envelope_h", 15, "mm", None, "altura de componentes, referencia"),
    ("# Ventilación bajo el UNO Q",),
    ("vent_width", 2.6, "mm", None, "ranuras longitudinales"),
    ("vent_offset", 2.4, "mm", None, "eje de ranura respecto al centro de celda"),
    ("vent_margin", 2, "mm", None, "a la nervadura"),
    ("# Tornillería M3 de expansión",),
    ("m3_clearance", 3.4, "mm", None, "pasante M3"),
    ("m3_boss_d", 7, "mm", None, ""),
    ("m3_nut_af", 5.7, "mm", None, "alojamiento tuerca M3 (5.5 + 0.2)"),
    ("m3_nut_depth", 2.8, "mm", None, ""),
    ("front_exp_x", 14, "mm", None, "2 puntos M3 frontales"),
    ("front_exp_spacing", 40, "mm", None, ""),
    ("lidar_cx", 45, "mm", None, "centro zona LiDAR 50x50"),
    ("lidar_cy", None, "mm", "=chassis_width / 2", ""),
    ("lidar_zone", 50, "mm", None, "zona plana reservada"),
    ("lidar_spacing_x", 40, "mm", None, "PROVISIONAL, ajustar al LiDAR elegido"),
    ("lidar_spacing_y", 40, "mm", None, "PROVISIONAL"),
    ("third_x0", 14, "mm", None, "4 puntos M3 para tercer nivel"),
    ("third_x1", None, "mm", "=chassis_length - third_x0", ""),
    ("third_y0", 12, "mm", None, ""),
    ("third_y1", None, "mm", "=chassis_width - third_y0", ""),
    ("# Cableado (zona potencia Y baja, zona señales Y alta)",),
    ("zip_w", 2.2, "mm", None, "ranura brida"),
    ("zip_l", 4.2, "mm", None, "bridas de hasta 3.6 mm"),
    ("zip_pitch", 7, "mm", None, "entre las dos ranuras del par"),
    ("zip_x1", 102.5, "mm", None, ""),
    ("zip_x2", 132, "mm", None, ""),
    ("pwr_hole_l", 16, "mm", None, "paso de cable de potencia A"),
    ("pwr_hole_w", 7, "mm", None, ""),
    ("pwr_a_x", 63, "mm", None, ""),
    ("pwr_b_l", 10, "mm", None, "paso de potencia B"),
    ("pwr_b_x", 115.5, "mm", None, ""),
    ("gpio_l", 20, "mm", None, "paso GPIO (cable plano / Dupont)"),
    ("gpio_w", 6, "mm", None, ""),
    ("gpio_x", 61.5, "mm", None, ""),
    ("usb_l", 11, "mm", None, "paso USB-C"),
    ("usb_w", 7, "mm", None, ""),
    ("usb_x", 116, "mm", None, ""),
    ("bus_l", 9, "mm", None, "pasos UART e I2C (franja trasera)"),
    ("bus_w", 5, "mm", None, ""),
    ("label_depth", 0.6, "mm", None, "texto grabado PWR / SIG"),
]


def build_params(doc):
    sh = doc.addObject("Spreadsheet::Sheet", "Params")
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
            val = eval(formula[1:], {}, vals)
            sh.set(cell, formula)
        else:
            sh.set(cell, ("%r %s" % (float(val), unit)).strip() if unit else repr(float(val)))
        sh.setAlias(cell, name)
        sh.set("C%d" % row, note)
        vals[name] = float(val)
        P[name] = E(val, "Params.%s" % name, unit == "mm")
    doc.recompute()
    return P, vals


# --------------------------------------------------------------------------- #
# Sketcher: perfiles totalmente restringidos y atados a la hoja
# --------------------------------------------------------------------------- #
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
            if "Params" in offset.x:
                s.o.setExpression(".AttachmentOffset.Base.z", offset.x)

    def _c(s, c, e=None):
        i = s.o.addConstraint(c)
        if e is not None and "Params" in e.x:
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
            if "Params" in v.x:
                f.setExpression(k, v.x)
        else:
            setattr(f, k, v)
    return f


# --------------------------------------------------------------------------- #
# Geometría derivada (posiciones)
# --------------------------------------------------------------------------- #
def layout(P):
    L = {}
    t = P["rib_thickness"]
    L["x0"], L["y0"] = P["deck_x0"], P["deck_y0"]
    L["x1"], L["y1"] = P["deck_x0"] + P["deck_length"], P["deck_y0"] + P["deck_width"]
    # soportes: centro entre los orificios index e index+1 del grupo
    supp = []
    for gi, gx in ((1, P["group_1_x"]), (2, P["group_2_x"])):
        xc = gx + (P["support_hole_index"] + 0.5) * P["square_hole_pitch"]
        for ri, ry in ((1, P["row_1_y"]), (2, P["row_2_y"])):
            supp.append(dict(name="Support_G%dR%d" % (gi, ri), g=gi, row=ri, xc=xc, yc=ry))
    L["supports"] = supp
    half = P["col_length"] / 2 + P["socket_clearance"] + t
    L["blocks"] = [(s_["xc"] - half, s_["xc"] + half) for s_ in supp if s_["row"] == 1]
    L["tab_y_lo"] = P["row_1_y"] - P["col_out"]
    L["tab_y_hi"] = P["row_2_y"] + P["col_out"]
    return L


def outline_pts(P, L):
    """Contorno CCW del piso con 4 orejetas para los soportes."""
    x0, y0, x1, y1 = L["x0"], L["y0"], L["x1"], L["y1"]
    (a0, a1), (b0, b1) = L["blocks"]
    lo, hi = L["tab_y_lo"], L["tab_y_hi"]
    R, r = P["deck_corner_r"], P["tab_corner_r"]
    pts = [((x0, y0), R),
           ((a0, y0), None), ((a0, lo), r), ((a1, lo), r), ((a1, y0), None),
           ((b0, y0), None), ((b0, lo), r), ((b1, lo), r), ((b1, y0), None),
           ((x1, y0), R), ((x1, y1), R),
           ((b1, y1), None), ((b1, hi), r), ((b0, hi), r), ((b0, y1), None),
           ((a1, y1), None), ((a1, hi), r), ((a0, hi), r), ((a0, y1), None),
           ((x0, y1), R)]
    return [p for p, _ in pts], [q for _, q in pts]


def cells(P, L):
    t = P["rib_thickness"]; h = t / 2
    xs = [P["rib_x_%d" % i] for i in range(1, 8)]
    ys = [P["rib_y_%d" % i] for i in range(1, 6)]
    xb = [(L["x0"] + t, xs[0] - h)] + [(xs[i] + h, xs[i + 1] - h) for i in range(6)] \
        + [(xs[6] + h, L["x1"] - t)]
    out = {}
    for ci, (cx0, cx1) in enumerate(xb):
        for rj in range(4):
            out[("c", ci, rj)] = (cx0, ys[rj] + h, cx1, ys[rj + 1] - h)
    (a0, a1), (b0, b1) = L["blocks"]
    segs = [("front", L["x0"] + t, xs[0] - h), ("s1b", xs[0] + h, a0), ("s2", a1, b0),
            ("s3", b1, xs[5] - h), ("s4", xs[5] + h, xs[6] - h), ("rear", xs[6] + h, L["x1"] - t)]
    for nm, sx0, sx1 in segs:
        out[("pwr", nm)] = (sx0, L["y0"] + t, sx1, ys[0] - h)
        out[("sig", nm)] = (sx0, ys[4] + h, sx1, L["y1"] - t)
    return out


def inside(c, x0, y0, x1, y1, m=0.3):
    return c[0].v + m <= x0 and c[1].v + m <= y0 and x1 <= c[2].v - m and y1 <= c[3].v - m


def find_cell(C, x0, y0, x1, y1):
    for k, c in C.items():
        if inside(c, x0, y0, x1, y1):
            return k
    return None


# --------------------------------------------------------------------------- #
# Piso superior
# --------------------------------------------------------------------------- #
def build_deck(doc, P, L, report):
    body = doc.addObject("PartDesign::Body", "Deck")
    zb = P["deck_height"]
    zt = P["deck_height"] + P["deck_thickness"]
    zr = P["deck_height"] - P["rib_height"]
    pts, radii = outline_pts(P, L)

    s = sk(body, "Sk_DeckOutline", offset=zb)
    s.poly(pts, radii)
    feat(body, "Pad", "Plate", s, Length=P["deck_thickness"])

    # nervaduras: contorno menos celdas, con islas (refuerzos) para tornillería
    C = cells(P, L)
    bosses = []
    m3r = P["m3_boss_d"] / 2
    cy = P["chassis_width"] / 2
    m3pts = {
        "front": [(P["front_exp_x"], cy - P["front_exp_spacing"] / 2),
                  (P["front_exp_x"], cy + P["front_exp_spacing"] / 2)],
        "lidar": [(P["lidar_cx"] + sx * P["lidar_spacing_x"] / 2, P["lidar_cy"] + sy * P["lidar_spacing_y"] / 2)
                  for sx in (-1, 1) for sy in (-1, 1)],
        "third": [(x, y) for x in (P["third_x0"], P["third_x1"]) for y in (P["third_y0"], P["third_y1"])],
    }
    for grp, lst in m3pts.items():
        for (x, y) in lst:
            bosses.append((grp, x, y, m3r))
    ux, uy = P["unoq_mount_offset_x"], P["unoq_mount_offset_y"]
    unopts = [(ux + sx * P["unoq_hole_spacing_x"] / 2, uy + sy * P["unoq_hole_spacing_y"] / 2)
              for sx in (-1, 1) for sy in (-1, 1)]
    for (x, y) in unopts:
        bosses.append(("unoq", x, y, P["unoq_boss_d"] / 2))

    used = {}
    for grp, x, y, r in bosses:
        k = find_cell(C, x.v - r.v, y.v - r.v, x.v + r.v, y.v + r.v)
        if k is None:
            raise RuntimeError("refuerzo %s en (%.1f, %.1f) cruza una nervadura: ajusta rib_x/rib_y"
                               % (grp, x.v, y.v))
        used.setdefault(k, []).append((x, y, r))

    s = sk(body, "Sk_Ribs", offset=zb)
    s.poly(pts, radii)
    for k, (x0, y0, x1, y1) in C.items():
        if x1.v - x0.v < 3 or y1.v - y0.v < 3:
            continue
        s.rect(x0, y0, x1, y1)
        for (x, y, r) in used.get(k, []):
            s.circle(x, y, r)
    feat(body, "Pad", "Ribs", s, Length=P["rib_height"], Reversed=True)

    # alojamientos de las columnas (abiertos hacia el borde)
    s = sk(body, "Sk_Sockets", offset=zr)
    c = P["socket_clearance"]
    for sp in L["supports"]:
        xa, xb_ = sp["xc"] - P["col_length"] / 2 - c, sp["xc"] + P["col_length"] / 2 + c
        if sp["row"] == 1:
            s.rect(xa, sp["yc"] - P["col_out"] - 2, xb_, sp["yc"] + P["col_in"] + c)
        else:
            s.rect(xa, sp["yc"] - P["col_in"] - c, xb_, sp["yc"] + P["col_out"] + 2)
    feat(body, "Pocket", "Sockets", s, Length=P["rib_height"], Reversed=True)

    # separadores del UNO Q

    # todos los pasantes, desde la cara superior
    s = sk(body, "Sk_Through", offset=zt)
    holes = []   # (tipo, x0, y0, x1, y1) para validar

    def rh(kind, cx, cy_, lx, ly, need_cell=True):
        s.rect_c(cx, cy_, lx, ly)
        holes.append((kind, cx.v - lx.v / 2, cy_.v - ly.v / 2, cx.v + lx.v / 2, cy_.v + ly.v / 2, need_cell))

    def ch(kind, cx, cy_, d, need_cell=False):
        s.circle(cx, cy_, d / 2)
        holes.append((kind, cx.v - d.v / 2, cy_.v - d.v / 2, cx.v + d.v / 2, cy_.v + d.v / 2, need_cell))

    for grp, lst in m3pts.items():
        for (x, y) in lst:
            ch("M3 " + grp, x, y, P["m3_clearance"])
    for sp in L["supports"]:
        ch("M2.5 soporte", sp["xc"], sp["yc"], P["m25_clearance"])
    ypw = (L["y0"] + P["rib_thickness"] + P["rib_y_1"] - P["rib_thickness"] / 2) / 2
    ysg = (L["y1"] - P["rib_thickness"] + P["rib_y_5"] + P["rib_thickness"] / 2) / 2
    # potencia (Y baja)
    rh("potencia A", P["pwr_a_x"], ypw, P["pwr_hole_l"], P["pwr_hole_w"])
    rh("potencia B", P["pwr_b_x"], ypw, P["pwr_b_l"], P["pwr_hole_w"] - 1)
    # señales (Y alta + franja trasera)
    rh("GPIO", P["gpio_x"], ysg, P["gpio_l"], P["gpio_w"])
    rh("USB", P["usb_x"], ysg, P["usb_l"], P["usb_w"])
    xr = (P["rib_x_7"] + P["rib_thickness"] / 2 + L["x1"] - P["rib_thickness"]) / 2
    yc2 = (P["rib_y_2"] + P["rib_y_3"]) / 2
    yc3 = (P["rib_y_3"] + P["rib_y_4"]) / 2
    rh("UART", xr, yc2, P["bus_w"], P["bus_l"])
    rh("I2C", xr, yc3, P["bus_w"], P["bus_l"])
    # paso frontal para cámara / pan-tilt
    xf = (L["x0"] + P["rib_thickness"] + P["rib_x_1"] - P["rib_thickness"] / 2) / 2
    rh("paso frontal", xf, yc3, P["bus_w"], P["bus_l"])
    # bridas: 2 pares en potencia, 2 en señales, 1 frontal (ranuras a lo largo de Y)
    for zx in (P["zip_x1"], P["zip_x2"]):
        for yy in (ypw, ysg):
            for sgn in (-1, 1):
                rh("brida", zx + sgn * P["zip_pitch"] / 2, yy, P["zip_w"], P["zip_l"])
    for sgn in (-1, 1):
        rh("brida frontal", xf, yc2 + sgn * P["zip_pitch"] / 2, P["zip_l"], P["zip_w"])
    # ventilación bajo el UNO Q: 2 ranuras longitudinales por celda
    vent_area = 0.0
    vent_cells = [(ci, rj) for ci in (4, 5) for rj in range(4)] + [(ci, rj) for ci in (3, 6) for rj in (1, 2)]
    for (ci, rj) in vent_cells:
        x0, y0, x1, y1 = C[("c", ci, rj)]
        cxv, cyv = (x0 + x1) / 2, (y0 + y1) / 2
        lx = x1 - x0 - 2 * P["vent_margin"]
        for sgn in (-1, 1):
            rh("ventilación", cxv, cyv + sgn * P["vent_offset"], lx, P["vent_width"])
            vent_area += lx.v * P["vent_width"].v
    feat(body, "Pocket", "ThroughHoles", s, Type="ThroughAll")

    # rosca para el UNO Q (atraviesa separador, placa y parte del refuerzo)
    s = sk(body, "Sk_UnoPilots", offset=zt)
    for (x, y) in unopts:
        s.circle(x, y, P["m25_pilot"] / 2)
    feat(body, "Pocket", "UnoPilots", s, Length=P["deck_thickness"] + P["rib_height"] - 1.2)

    # alojamientos hexagonales para tuercas M3 desde abajo
    s = sk(body, "Sk_NutTraps", offset=zr)
    for grp, lst in m3pts.items():
        for (x, y) in lst:
            s.hexagon(x, y, P["m3_nut_af"])
    feat(body, "Pocket", "NutTraps", s, Length=P["m3_nut_depth"], Reversed=True)

    # rótulos grabados de zona
    import Draft
    font = "/System/Library/Fonts/Supplemental/Arial Bold.ttf"
    labels = []
    if os.path.exists(font):
        xy = [pl for pl in body.Origin.OriginFeatures if pl.Role == "XY_Plane"][0]
        a0 = L["blocks"][0][0]
        for txt, yy in (("PWR", L["tab_y_lo"] + 4.6), ("SIG", L["tab_y_hi"] - 8.6)):
            ss = Draft.make_shapestring(txt, font, 4.0)
            body.addObject(ss)
            ss.AttachmentSupport = [(xy, "")]
            ss.MapMode = "FlatFace"
            ox = a0 + 5.0
            ss.AttachmentOffset = App.Placement(V(ox.v, yy.v, zt.v), App.Rotation())
            ss.setExpression(".AttachmentOffset.Base.x", ox.x)
            ss.setExpression(".AttachmentOffset.Base.y", yy.x)
            ss.setExpression(".AttachmentOffset.Base.z", zt.x)
            labels.append(feat(body, "Pocket", "Label_" + txt, ss, Length=P["label_depth"]))

    report["holes"] = holes
    report["cells"] = C
    report["vent_area"] = vent_area
    report["m3pts"] = {k: [(x.v, y.v) for x, y in v] for k, v in m3pts.items()}
    report["unopts"] = [(x.v, y.v) for x, y in unopts]
    return body


# --------------------------------------------------------------------------- #
# Soporte (coordenadas locales: origen entre los 2 orificios, +Y hacia dentro)
# --------------------------------------------------------------------------- #
def build_support(doc, P):
    body = doc.addObject("PartDesign::Body", "Support")
    hp = P["square_hole_pitch"] / 2
    zero = E.k(0)
    yo, yi = -P["col_out"], P["col_in"]

    s = sk(body, "Sk_Foot")
    s.rect(-P["foot_length"] / 2, yo, P["foot_length"] / 2, P["foot_in"], P["col_corner_r"])
    feat(body, "Pad", "Foot", s, Length=P["foot_thickness"])

    s1 = sk(body, "Sk_LoftBase", offset=P["foot_thickness"])
    s1.rect(-P["foot_length"] / 2, yo, P["foot_length"] / 2, P["foot_in"], P["col_corner_r"])
    s2 = sk(body, "Sk_LoftTop", offset=P["foot_thickness"] + P["foot_transition"])
    s2.rect(-P["col_length"] / 2, yo, P["col_length"] / 2, yi, P["col_corner_r"])
    loft = body.newObject("PartDesign::AdditiveLoft", "FootTransition")
    loft.Profile = s1.finish()
    loft.Sections = [s2.finish()]
    loft.Ruled = True

    s = sk(body, "Sk_Column")
    s.rect(-P["col_length"] / 2, yo, P["col_length"] / 2, yi, P["col_corner_r"])
    feat(body, "Pad", "Column", s, Length=P["deck_height"])

    # nervaduras hacia el interior: plano YZ (x de sketch = Y, y de sketch = Z)
    for i, off in enumerate((-P["gusset_pitch"], zero, P["gusset_pitch"])):
        s = sk(body, "Sk_Gusset%d" % i, plane="YZ_Plane", offset=off)
        ft = P["foot_thickness"]
        s.poly([(yi - 0.5, ft), (P["foot_in"] - 0.5, ft), (yi - 0.5, P["gusset_height"])])
        feat(body, "Pad", "Gusset%d" % i, s, Length=P["gusset_thickness"], SideType="Symmetric")

    # guía cuadrada (orificio index) con punta cónica de entrada
    s = sk(body, "Sk_Locator")
    s.rect_c(-hp, zero, P["locator_size"], P["locator_size"])
    feat(body, "Pad", "Locator", s, Length=P["locator_height"], Reversed=True, TaperAngle=-6.0)

    # tornillo M2.5 desde abajo por el orificio index+1
    s = sk(body, "Sk_BottomPilot")
    s.circle(hp, zero, P["m25_pilot"] / 2)
    feat(body, "Pocket", "BottomPilot", s, Length=P["pilot_depth"], Reversed=True)

    # tornillo M2.5 desde arriba (fija el piso)
    s = sk(body, "Sk_TopPilot", offset=P["deck_height"])
    s.circle(zero, zero, P["m25_pilot"] / 2)
    feat(body, "Pocket", "TopPilot", s, Length=P["pilot_depth"])
    return body


# --------------------------------------------------------------------------- #
# Referencias: chasis y envolvente del UNO Q
# --------------------------------------------------------------------------- #
def build_chassis_ref(doc, P):
    body = doc.addObject("PartDesign::Body", "ChassisRef")
    s = sk(body, "Sk_Chassis")
    s.rect(E.k(0), E.k(0), P["chassis_length"], P["chassis_width"])
    for gx in (P["group_1_x"], P["group_2_x"]):
        for i in range(int(P["holes_per_group"].v)):
            for ry in (P["row_1_y"], P["row_2_y"]):
                s.rect_c(gx + i * P["square_hole_pitch"], ry, P["square_hole_size"], P["square_hole_size"])
    feat(body, "Pad", "ChassisPlate", s, Length=P["chassis_thickness"], Reversed=True)
    return body


def build_unoq_env(doc, P):
    body = doc.addObject("PartDesign::Body", "UnoQ_Env")
    z = P["deck_height"] + P["deck_thickness"] + P["standoff_height"]
    s = sk(body, "Sk_UnoQ", offset=z)
    s.rect_c(P["unoq_mount_offset_x"], P["unoq_mount_offset_y"], P["unoq_length"], P["unoq_width"])
    feat(body, "Pad", "UnoQBox", s, Length=P["unoq_envelope_h"])
    return body


# --------------------------------------------------------------------------- #
# Fillets en el árbol (bordes seleccionados por geometría al construir)
# --------------------------------------------------------------------------- #
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


def main():
    args = argv_script()
    out = os.path.abspath(args[args.index("--out") + 1]) if "--out" in args else os.getcwd()
    os.makedirs(out, exist_ok=True)

    doc = App.newDocument("rover_deck_v1")
    P, vals = build_params(doc)
    L = layout(P)
    report = {}
    deck = build_deck(doc, P, L, report)
    support = build_support(doc, P)
    chassis = build_chassis_ref(doc, P)
    unoq = build_unoq_env(doc, P)
    doc.recompute()

    zt = (P["deck_height"] + P["deck_thickness"]).v
    zb = P["deck_height"].v
    # chaflán en el borde de la cara superior: es la que apoya en la cama (pata de elefante)
    fr = []
    outer = deck.Tip.Shape.Faces
    topf = [fc for fc in outer if abs(fc.BoundBox.ZMin - zt) < 1e-4 and abs(fc.BoundBox.ZMax - zt) < 1e-4]
    top = max(topf, key=lambda fc: fc.Area)
    ow = top.OuterWire
    f, n = try_fillet(doc, deck, "Chamfer_BedEdge",
                      lambda e: any(e.isSame(o) for o in ow.Edges), 0.5, kind="Chamfer")
    fr.append(("chaflán 0.5 en el borde que apoya en la cama", f is not None, n))

    # 4 soportes por App::Link, colocados por expresión
    links = []
    for sp in L["supports"]:
        lk = doc.addObject("App::Link", sp["name"])
        lk.LinkedObject = support
        rot = 0 if sp["row"] == 1 else 180
        lk.Placement = App.Placement(V(sp["xc"].v, sp["yc"].v, 0), App.Rotation(V(0, 0, 1), rot))
        lk.setExpression(".Placement.Base.x", sp["xc"].x)
        lk.setExpression(".Placement.Base.y", sp["yc"].x)
        links.append((lk, sp))
    verificar(doc)

    # -------------------------- validaciones -------------------------------- #
    R = []
    def chk(name, ok, detail):
        R.append((name, "OK" if ok is True else ("FALLA" if ok is False else ok), detail))

    sk_bad = [o.o.Name for o in SKETCHES if not o.o.FullyConstrained
              or o.o.ConflictingConstraints or o.o.RedundantConstraints]
    nexpr = sum(len(o.ExpressionEngine) for o in doc.Objects if hasattr(o, "ExpressionEngine"))
    ds, ss_ = deck.Shape, support.Shape
    bb = ds.optimalBoundingBox()
    chk("1. Huella del piso <= chasis",
        bb.XMin >= 0 and bb.YMin >= 0 and bb.XMax <= vals["chassis_length"] and bb.YMax <= vals["chassis_width"],
        "X %.2f..%.2f, Y %.2f..%.2f (%.2f x %.2f mm, chasis %g x %g)"
        % (bb.XMin, bb.XMax, bb.YMin, bb.YMax, bb.XLength, bb.YLength, vals["chassis_length"], vals["chassis_width"]))

    placed = []
    for lk, sp in links:
        sh = support.Shape.copy(); sh.Placement = lk.Placement
        placed.append(sh)
    pos = "; ".join("%s (%.3f, %.2f)" % (sp["name"], sp["xc"].v, sp["yc"].v) for _, sp in links)
    holes_c = chassis.Shape
    inter_ch = sum(sh.common(holes_c).Volume for sh in placed)
    chk("2. Posición de soportes", inter_ch < 1e-3,
        "centros %s; interferencia soportes-chasis %.4f mm3 (guía y tornillo dentro de los orificios)" % (pos, inter_ch))
    gap = (vals["square_hole_size"] - vals["locator_size"]) / 2
    chk("3. Guía 2.55 en orificio 2.89", gap >= 0.1,
        "holgura %.3f mm por lado (%.2f total); punta cónica 6°; altura %.1f mm vs acrílico %.1f mm (SUPUESTO)"
        % (gap, 2 * gap, vals["locator_height"], vals["chassis_thickness"]))
    inter_dk = sum(sh.common(ds).Volume for sh in placed)
    chk("2b. Columnas en sus alojamientos", inter_dk < 1e-3,
        "interferencia soporte-piso %.4f mm3; holgura lateral %.2f mm por lado; la cara superior de la "
        "columna apoya contra la placa (contacto previsto)" % (inter_dk, vals["socket_clearance"]))
    allb = ds.fuse(placed).optimalBoundingBox()
    chk("4/5. Ruedas y motores", "NO VERIFICABLE",
        "sin geometría de ruedas/motores. Todo queda dentro de la huella del chasis (X %.2f..%.2f, Y %.2f..%.2f) "
        "y por encima de Z=0 salvo las guías (%.1f mm dentro del acrílico)"
        % (allb.XMin, allb.XMax, allb.YMin, allb.YMax, vals["locator_height"]))
    ub = unoq.Shape.optimalBoundingBox()
    i_uno = unoq.Shape.common(ds).Volume
    m_front = ub.XMin - (vals["lidar_cx"] + vals["lidar_zone"] / 2)
    m_rear = bb.XMax - ub.XMax
    chk("6. Acceso al UNO Q", i_uno < 1e-3 and ub.XMax <= bb.XMax and ub.YMin >= bb.YMin,
        "sin interferencia (%.4f mm3); cara superior libre; bordes: frente %.1f mm a la zona LiDAR, trasera %.1f mm al borde, "
        "laterales sobre las franjas de cableado; patrón de agujeros PROVISIONAL" % (i_uno, m_front, m_rear))

    # espesores mínimos: distancia entre pasantes y a las nervaduras
    holes = report["holes"]; C = report["cells"]
    dmin, pair = 1e9, None
    for i in range(len(holes)):
        for j in range(i + 1, len(holes)):
            a, b = holes[i], holes[j]
            dx = max(b[1] - a[3], a[1] - b[3], 0); dy = max(b[2] - a[4], a[2] - b[4], 0)
            d = math.hypot(dx, dy)
            if d < dmin:
                dmin, pair = d, (a[0], b[0])
    cut_ribs = [h[0] for h in holes if h[5] and find_cell(C, h[1], h[2], h[3], h[4]) is None]
    walls = {
        "placa": vals["deck_thickness"],
        "nervadura": vals["rib_thickness"],
        "pared piloto inferior (Y)": vals["col_out"] - vals["m25_pilot"] / 2,
        "pared piloto inferior (X)": vals["col_length"] / 2 - vals["square_hole_pitch"] / 2 - vals["m25_pilot"] / 2,
        "pared alojamiento columna": vals["rib_thickness"],
        "suelo bajo tuerca M3": vals["rib_height"] + vals["deck_thickness"] - vals["m3_nut_depth"],
    }
    wmin = min(walls.values())
    chk("7. Espesores mínimos", wmin >= 1.2 and dmin >= 1.2 and not cut_ribs,
        "pared mínima %.2f mm (%s); separación mínima entre pasantes %.2f mm (%s / %s); pasantes que cortan nervaduras: %s"
        % (wmin, min(walls, key=walls.get), dmin, pair[0], pair[1], cut_ribs or "ninguno"))
    chk("8. Ventilación", report["vent_area"] > 0,
        "%d ranuras, %.0f mm2 de paso bajo la placa (%.0f %% de la huella del UNO Q)"
        % (sum(1 for h in holes if h[0] == "ventilación"), report["vent_area"],
           100 * report["vent_area"] / (vals["unoq_length"] * vals["unoq_width"])))

    # imprimibilidad: piso boca abajo (cara superior plana sobre la cama) y soporte tumbado
    deck_print = ds.copy(); deck_print.rotate(V(0, 0, 0), V(1, 0, 0), 180)
    deck_print.translate(V(0, 0, -deck_print.optimalBoundingBox().ZMin))
    sup_print = ss_.copy(); sup_print.rotate(V(0, 0, 0), V(1, 0, 0), 90)
    sup_print.translate(V(0, 0, -sup_print.optimalBoundingBox().ZMin))
    print("\n== informe de impresión: piso (boca abajo, cara plana en la cama) ==")
    informe_impresion(deck_print)
    print("\n== informe de impresión: soporte (cara exterior abajo) ==")
    informe_impresion(sup_print)

    def overhangs(shape, lim=-0.72):
        z0 = shape.optimalBoundingBox().ZMin; bad = []
        for fc in shape.Faces:
            pr = fc.ParameterRange
            try:
                n = fc.normalAt((pr[0] + pr[1]) / 2, (pr[2] + pr[3]) / 2)
            except Exception:
                continue
            bb_ = fc.optimalBoundingBox()
            if n.z < lim and fc.Area > 1 and abs(bb_.ZMax - z0) > 1e-3:
                bad.append(min(bb_.XLength, bb_.YLength))
        return bad
    ov_d, ov_s = overhangs(deck_print), overhangs(sup_print)
    chk("9. Imprimible sin soportes", max(ov_d + ov_s + [0]) <= 20.0,
        "piso: %d caras colgantes, máx %.1f mm (fondo de rótulos grabados y de pilotos ciegos Ø2.2); "
        "soporte: %d caras colgantes, máx %.1f mm (guía de %.1f mm y techos de pilotos)"
        % (len(ov_d), max(ov_d + [0]), len(ov_s), max(ov_s + [0]), vals["locator_height"]))
    chk("10. Editable", not sk_bad,
        "%d sketches, todos totalmente restringidos%s; %d expresiones atadas a Params"
        % (len(SKETCHES), "" if not sk_bad else " SALVO " + ", ".join(sk_bad), nexpr))

    print("\n== validaciones ==")
    for n_, st, d in R:
        print("[%s] %s: %s" % (st, n_, d))
    print("\nredondeos:", fr)
    print("piso   : %.1f cm3, ~%.0f g PLA (sólido 100 %%)" % (ds.Volume / 1000, masa(ds)))
    print("soporte: %.1f cm3, ~%.1f g PLA c/u" % (ss_.Volume / 1000, masa(ss_)))

    # ----------------------------- exportación ------------------------------ #
    for nm, shp in (("deck", deck_print), ("support", sup_print)):
        m, probs = validar_para_impresion(shp)
        m.write(os.path.join(out, "rover_deck_v1_%s.stl" % nm))
        if probs:
            raise RuntimeError("malla con problemas: %s" % probs)
    Import.export([deck], os.path.join(out, "rover_deck_v1_deck.step"))
    Import.export([support], os.path.join(out, "rover_deck_v1_support.step"))
    asm = [deck] + [lk for lk, _ in links]
    Import.export(asm, os.path.join(out, "rover_deck_v1_assembly.step"))
    # al abrir en la GUI: sólo las piezas finales (Tip de cada cuerpo y los Links)
    for b in (deck, support, chassis, unoq):
        for o in b.Group:
            o.Visibility = (o == b.Tip)
        b.Origin.Visibility = False
        b.Visibility = b is not support   # el original queda en el origen; se ven sus Links
    for lk, _ in links:
        lk.Visibility = True
    doc.saveAs(os.path.join(out, "rover_deck_v1.FCStd"))
    with open(os.path.join(out, "validation.json"), "w") as fh:
        json.dump({"checks": R, "fillets": fr,
                   "deck_cm3": ds.Volume / 1000, "support_cm3": ss_.Volume / 1000,
                   "deck_bbox": [bb.XMin, bb.YMin, bb.ZMin, bb.XMax, bb.YMax, bb.ZMax],
                   "m3_points": report["m3pts"], "unoq_points": report["unopts"],
                   "supports": [(sp["name"], sp["xc"].v, sp["yc"].v) for _, sp in links]},
                  fh, indent=1, ensure_ascii=False)
    print("\nescrito en", out)


ejecutar(main)
