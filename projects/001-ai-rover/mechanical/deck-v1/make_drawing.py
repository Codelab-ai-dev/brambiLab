"""Planos 2D acotados del piso superior y del soporte, a partir del modelo.

  fc.sh run make_drawing.py -- --model out/rover_deck_v1.FCStd --out out/

Escribe tres hojas, cada una como .svg y como .FCStd (página TechDraw):
  rover_deck_v1_plano_piso        A3 1:1  vistas superior, inferior, lateral, frontal e isométrica
  rover_deck_v1_plano_posiciones  A3 1:1  cotas ordenadas de todas las interfaces y tabla
  rover_deck_v1_plano_soporte     A4 2:1  vistas del soporte
Las vistas son proyecciones reales del sólido; las cotas se calculan con los
valores de la hoja Params. El PDF se exporta en modo app con export_pdf.py.
"""
import sys, os
sys.path.insert(0, os.path.expanduser("~/.claude/skills/freecad/scripts"))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from fchelpers import *  # noqa: F401,F403
from sheetlib import View, dim, txt, title_block, write_sheet


def params(doc):
    sh = doc.getObject("Params")
    def p(n):
        v = sh.get(n)
        return float(v.Value) if hasattr(v, "Value") else float(v)
    return p


# --------------------------------------------------------------------------- #
def sheet_deck(doc, p, out):
    W, H = 420.0, 297.0
    deck = doc.getObject("Deck").Shape
    zb, zt = p("deck_height"), p("deck_height") + p("deck_thickness")
    zr, zs = zb - p("rib_height"), zt
    x0, x1 = p("deck_x0"), p("deck_x0") + p("deck_length")
    y0, y1 = p("deck_y0"), p("deck_y0") + p("deck_width")
    ylo, yhi = p("row_1_y") - p("col_out"), p("row_2_y") + p("col_out")
    g = []
    top = View(deck, "top", 1.0, 105, 80, hidden=False)
    bot = View(deck, "bottom", 1.0, 290, 80, hidden=False)
    fr = View(deck, "front", 1.0, 122, 162)
    rt = View(deck, "right", 1.0, 290, 162)
    iso = View(deck, "iso", 0.8, 115, 228, hidden=False)
    for v, name in ((top, "VISTA SUPERIOR"), (bot, "VISTA INFERIOR (nervaduras)"), (fr, "VISTA LATERAL"),
                    (rt, "VISTA FRONTAL (desde atrás, X+)"), (iso, "VISTA ISOMÉTRICA (0.8:1)")):
        g += v.svg()
        g.append(txt((v.left_x() + v.right_x()) / 2, v.top_y() - (14 if v is top else 4), name, 3, cls="h"))
    # superior: largo, ancho, ancho con orejetas
    g += dim(top.M((x0, y0, zt)), top.M((x1, y0, zt)), top.bottom_y() + 8, p("deck_length"))
    g += dim(top.M((x0, y1, zt)), top.M((x0, y0, zt)), top.left_x() - 6, p("deck_width"), horizontal=False)
    a0 = p("group_1_x") + (p("support_hole_index") + 0.5) * p("square_hole_pitch") - p("col_length") / 2 \
        - p("socket_clearance") - p("rib_thickness")
    g += dim(top.M((a0, yhi, zt)), top.M((a0, ylo, zt)), top.left_x() - 13, yhi - ylo, horizontal=False)
    g += dim(top.M((x0, y1, zt)), top.M((a0, y1, zt)), top.top_y() - 4, a0 - x0)
    g += dim(top.M((a0, y1, zt)), top.M((a0 + 2 * (p("col_length") / 2 + p("socket_clearance") + p("rib_thickness")), y1, zt)),
             top.top_y() - 4, 2 * (p("col_length") / 2 + p("socket_clearance") + p("rib_thickness")))
    # lateral: alturas
    g += dim(fr.M((x1, 0, zr)), fr.M((x1, 0, zs)), fr.right_x() + 7, zs - zr, horizontal=False)
    g += dim(fr.M((x0, 0, zr)), fr.M((x0, 0, zb)), fr.left_x() - 5, p("rib_height"), horizontal=False)
    g += dim(fr.M((x0, 0, zb)), fr.M((x0, 0, zt)), fr.left_x() - 14, p("deck_thickness"), horizontal=False)
    g.append(txt(fr.left_x(), fr.bottom_y() + 6,
                 "Cara inferior de la placa a %g mm sobre el chasis (deck_height). Imprimir boca abajo: cara superior en la cama."
                 % zb, 2.4, "start"))
    g += dim(rt.M((0, ylo, zt)), rt.M((0, yhi, zt)), rt.bottom_y() + 8, yhi - ylo)
    notes = [
        "Pasantes M3 Ø%g con alojamiento de tuerca hexagonal %g e/c x %g desde abajo." % (p("m3_clearance"), p("m3_nut_af"), p("m3_nut_depth")),
        "UNO Q: separadores comerciales M2.5 x %g roscados en piloto Ø%g. Fijación del piso: M2.5 pasante Ø%g por orejeta."
        % (p("standoff_height"), p("m25_pilot"), p("m25_clearance")),
        "Imprimir boca abajo (cara superior plana en la cama, chaflán 0.5 en el borde): sin soportes ni puentes.",
        "Posiciones de cada interfaz: ver hoja BL-DECK-002.",
    ]
    g += title_block(W, H, "Piso superior v1", "BL-DECK-001", "1:1", "1/3", notes)
    return write_sheet(os.path.join(out, "rover_deck_v1_plano_piso"), W, H, g, "PlanoPiso")


# --------------------------------------------------------------------------- #
def sheet_positions(doc, p, out):
    W, H = 420.0, 297.0
    OX, OY = 45.0, 150.0
    deck = doc.getObject("Deck").Shape
    chassis = doc.getObject("ChassisRef").Shape
    links = [o for o in doc.Objects if o.TypeId == "App::Link" and o.Name.startswith("Support_")]
    zt = p("deck_height") + p("deck_thickness")
    zr = p("deck_height") - p("rib_height") + 0.5

    def P(x, y):
        return OX + x, OY - y

    def path_of(w):
        pts = w.discretize(Deflection=0.02)
        return "M %.3f %.3f " % P(pts[0].x, pts[0].y) + " ".join("L %.3f %.3f" % P(q.x, q.y) for q in pts[1:]) + " Z"

    g = []
    for w in chassis.slice(V(0, 0, 1), -p("chassis_thickness") / 2):
        g.append('<path d="%s" class="chassis"/>' % path_of(w))
    for w in deck.slice(V(0, 0, 1), zr):
        g.append('<path d="%s" class="hid"/>' % path_of(w))
    for lk in links:
        s = doc.getObject("Support").Shape.copy(); s.Placement = lk.Placement
        for w in s.slice(V(0, 0, 1), 1.5):
            g.append('<path d="%s" class="support"/>' % path_of(w))
    for z in (zt - 0.7, zt + 1.5):     # bajo el grabado de 0.6 mm, y separadores
        for w in deck.slice(V(0, 0, 1), z):
            g.append('<path d="%s" class="vis"/>' % path_of(w))

    lx, ly, lz = p("lidar_cx"), p("lidar_cy"), p("lidar_zone")
    xa, ya = P(lx - lz / 2, ly + lz / 2)
    g.append('<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" class="zone"/>' % (xa, ya, lz, lz))
    g.append(txt(*P(lx, ly - 2), "ZONA LIDAR %gx%g" % (lz, lz), 2.4, cls="z"))
    ux, uy, ul, uw = p("unoq_mount_offset_x"), p("unoq_mount_offset_y"), p("unoq_length"), p("unoq_width")
    xa, ya = P(ux - ul / 2, uy + uw / 2)
    g.append('<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" class="board"/>' % (xa, ya, ul, uw))
    g.append(txt(*P(ux, uy + uw / 2 + 1.2), "UNO Q (envolvente PROVISIONAL)", 2.2, cls="z"))
    g.append(txt(*P(p("pwr_b_x"), 1.5), "ZONA POTENCIA", 2.2, cls="z"))
    g.append(txt(*P(p("pwr_b_x"), p("chassis_width") - 3.2), "ZONA SEÑALES", 2.2, cls="z"))
    g.append(txt(OX - 30, OY - p("chassis_width") / 2, "FRENTE (supuesto) →", 3, rot=-90, cls="z"))

    def ord_x(x, lane):
        xs, ys = P(x, 0)
        yb = OY + 6 + lane * 13
        g.append('<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" class="dim"/>' % (xs, ys + 1, xs, yb))
        g.append(txt(xs + 0.9, yb + 0.8, "%g" % round(x, 3), 2.2, "start", rot=90))

    def ord_y(y, lane):
        xs, ys = P(0, y)
        xb = OX - 3 - lane * 11
        g.append('<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" class="dim"/>' % (xs - 1, ys, xb, ys))
        g.append(txt(xb - 0.8, ys + 0.8, "%g" % round(y, 3), 2.2, "end"))

    cy = p("chassis_width") / 2
    supp = sorted({round(lk.Placement.Base.x, 3) for lk in links})
    lanes_x = [
        [0, p("deck_x0")] + supp + [p("deck_x0") + p("deck_length"), p("chassis_length")],
        [p("front_exp_x"), lx - p("lidar_spacing_x") / 2, lx + p("lidar_spacing_x") / 2, p("third_x1")],
        [ux - p("unoq_hole_spacing_x") / 2, ux, ux + p("unoq_hole_spacing_x") / 2],
    ]
    lanes_y = [
        [0, p("row_1_y"), cy, p("row_2_y"), p("chassis_width")],
        [p("deck_y0"), p("third_y0"), cy - p("lidar_spacing_y") / 2, cy + p("lidar_spacing_y") / 2,
         p("third_y1"), p("deck_y0") + p("deck_width")],
        [uy - p("unoq_hole_spacing_y") / 2, uy + p("unoq_hole_spacing_y") / 2],
    ]
    for i, lane in enumerate(lanes_x):
        for x in lane:
            ord_x(x, i)
    for i, lane in enumerate(lanes_y):
        for y in lane:
            ord_y(y, i)
    g.append(txt(OX, OY + 50, "Cotas ordenadas en mm desde la esquina del chasis (0,0). Línea 1: chasis, piso y soportes; "
                 "línea 2: LiDAR, expansión y tercer nivel; línea 3: UNO Q.", 2.4, "start"))

    rows = [("Interfaz", "X", "Y", "Tornillo", "Nota")]
    for lk in sorted(links, key=lambda o: o.Name):
        b = lk.Placement.Base
        rows.append((lk.Name, "%.3f" % b.x, "%.2f" % b.y, "M2.5", "guía 2.55 + tornillo"))
    for y in (cy - p("front_exp_spacing") / 2, cy + p("front_exp_spacing") / 2):
        rows.append(("Expansión frontal", "%g" % p("front_exp_x"), "%g" % y, "M3 Ø3.4", "tuerca abajo"))
    for sx in (-1, 1):
        for sy in (-1, 1):
            rows.append(("LiDAR", "%g" % (lx + sx * p("lidar_spacing_x") / 2), "%g" % (ly + sy * p("lidar_spacing_y") / 2),
                         "M3 Ø3.4", "PROVISIONAL"))
    for x in (p("third_x0"), p("third_x1")):
        for y in (p("third_y0"), p("third_y1")):
            rows.append(("Tercer nivel", "%g" % x, "%g" % y, "M3 Ø3.4", "tuerca abajo"))
    for sx in (-1, 1):
        for sy in (-1, 1):
            rows.append(("UNO Q", "%g" % (ux + sx * p("unoq_hole_spacing_x") / 2), "%g" % (uy + sy * p("unoq_hole_spacing_y") / 2),
                         "M2.5 (Ø2.2)", "PROVISIONAL"))
    tx, ty = 236.0, 24.0
    cols = [0, 34, 52, 68, 92]
    g.append(txt(tx, ty - 6, "TABLA DE INTERFACES (coordenadas del chasis)", 3.2, "start", cls="h"))
    for i, r in enumerate(rows):
        for c, v in zip(cols, r):
            g.append(txt(tx + c, ty + i * 5.0, v, 2.4, "start", cls="h" if i == 0 else "t"))
        g.append('<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" class="grid"/>' % (tx, ty + i * 5.0 + 1.6, tx + 165, ty + i * 5.0 + 1.6))
    legend = [
        "Línea gruesa: piso (cara superior y separadores). Discontinua: nervaduras.",
        "Punto-raya: placa del chasis y orificios cuadrados. Gris: huella de los soportes.",
        "Orificios del chasis %g mm, paso %g, grupos X=%g y %g, filas Y=%g y %g."
        % (p("square_hole_size"), p("square_hole_pitch"), p("group_1_x"), p("group_2_x"), p("row_1_y"), p("row_2_y")),
        "Soporte: guía en el orificio %d y M2.5 desde abajo en el %d de cada grupo."
        % (p("support_hole_index") + 1, p("support_hole_index") + 2),
        "PROVISIONAL: patrón UNO Q y LiDAR, espesor del acrílico (%g mm)." % p("chassis_thickness"),
    ]
    for i, n in enumerate(legend):
        g.append(txt(236, 150 + i * 5, n, 2.4, "start"))
    g += title_block(W, H, "Piso superior v1 — posiciones", "BL-DECK-002", "1:1", "2/3")
    return write_sheet(os.path.join(out, "rover_deck_v1_plano_posiciones"), W, H, g, "PlanoPosiciones")


# --------------------------------------------------------------------------- #
def sheet_support(doc, p, out):
    W, H = 297.0, 210.0
    sup = doc.getObject("Support").Shape
    s = 2.0
    g = []
    fr = View(sup, "front", s, 62, 72)
    rt = View(sup, "left", s, 128, 72)
    tp = View(sup, "top", s, 62, 152)
    bt = View(sup, "bottom", s, 128, 152)
    iso = View(sup, "iso", 1.6, 222, 80, hidden=False)
    for v, name in ((fr, "VISTA EXTERIOR"), (rt, "VISTA LATERAL"), (tp, "VISTA SUPERIOR"),
                    (bt, "VISTA INFERIOR"), (iso, "ISOMÉTRICA (1.6:1)")):
        g += v.svg()
        g.append(txt((v.left_x() + v.right_x()) / 2, v.top_y() - 10, name, 2.8, cls="h"))
    L, hp = p("foot_length") / 2, p("square_hole_pitch") / 2
    Hh, ft, lh = p("deck_height"), p("foot_thickness"), p("locator_height")
    yo, yi, fi = -p("col_out"), p("col_in"), p("foot_in")
    c = p("col_length") / 2
    g += dim(fr.M((-L, yo, 0)), fr.M((L, yo, 0)), fr.bottom_y() + 5, p("foot_length"))
    g += dim(fr.M((-c, yo, Hh)), fr.M((c, yo, Hh)), fr.top_y() - 3, p("col_length"))
    g += dim(fr.M((L, yo, 0)), fr.M((L, yo, Hh)), fr.right_x() + 5, Hh, horizontal=False)
    g += dim(fr.M((-L, yo, 0)), fr.M((-L, yo, ft)), fr.left_x() - 4, ft, horizontal=False)
    g += dim(fr.M((-L, yo, -lh)), fr.M((-L, yo, 0)), fr.left_x() - 10, lh, horizontal=False)
    g += dim(rt.M((0, yo, 0)), rt.M((0, fi, 0)), rt.bottom_y() + 5, fi - yo)
    g += dim(rt.M((0, yo, Hh)), rt.M((0, yi, Hh)), rt.top_y() - 3, yi - yo)
    g += dim(rt.M((0, yi, 0)), rt.M((0, yi, p("gusset_height"))), rt.right_x() + 5, p("gusset_height"), horizontal=False)
    g += dim(bt.M((-hp, 0, 0)), bt.M((hp, 0, 0)), bt.bottom_y() + 5, p("square_hole_pitch"))
    g += dim(bt.M((-hp - p("locator_size") / 2, 0, 0)), bt.M((-hp + p("locator_size") / 2, 0, 0)),
             bt.top_y() - 3, p("locator_size"), label="□%g" % p("locator_size"))
    notes = [
        "Guía cuadrada %g (orificio %g): holgura %.2f por lado; punta cónica 6°." %
        (p("locator_size"), p("square_hole_size"), (p("square_hole_size") - p("locator_size")) / 2),
        "Pilotos Ø%g x %g para M2.5: abajo (chasis) y arriba (piso)." % (p("m25_pilot"), p("pilot_depth")),
        "Imprimir tumbado sobre la cara exterior plana; 4 piezas iguales.",
        "Altura = deck_height (35-45 mm, hoja Params).",
    ]
    g += title_block(W, H, "Soporte del piso v1 (x4)", "BL-DECK-003", "2:1", "3/3", notes, notes_xy=(172, 132))
    return write_sheet(os.path.join(out, "rover_deck_v1_plano_soporte"), W, H, g, "PlanoSoporte")


def main():
    args = argv_script()
    model = os.path.abspath(args[args.index("--model") + 1])
    out = os.path.abspath(args[args.index("--out") + 1])
    doc = App.openDocument(model)
    doc.recompute()
    p = params(doc)
    for f in (sheet_deck, sheet_positions, sheet_support):
        App.setActiveDocument(doc.Name)
        print("escrito", f(doc, p, out))


ejecutar(main)
