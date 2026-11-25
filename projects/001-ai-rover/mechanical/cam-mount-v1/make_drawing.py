"""Plano acotado del soporte de cámara (A3). Usa sheetlib del piso v1.

  fc.sh run make_drawing.py -- --model out/ESP32_CAM_Rover_Mount.FCStd --out out/
  fc.sh gui ../deck-v1/export_pdf.py -- --dir out     (no: usa export_pdf.py de esta carpeta)
"""
import sys, os
sys.path.insert(0, os.path.expanduser("~/.claude/skills/freecad/scripts"))
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "deck-v1"))
from fchelpers import *  # noqa: F401,F403
from sheetlib import View, dim, txt, title_block, write_sheet


def main():
    args = argv_script()
    model = os.path.abspath(args[args.index("--model") + 1])
    out = os.path.abspath(args[args.index("--out") + 1])
    doc = App.openDocument(model)
    doc.recompute()
    sh = doc.getObject("ROVER_CAM_PARAMETERS")
    def p(n):
        v = sh.get(n)
        return float(v.Value) if hasattr(v, "Value") else float(v)
    W, H = 420.0, 297.0
    car = doc.getObject("Carrier").Shape
    arm = doc.getObject("Arm").Shape
    g = []
    s2 = 2.0
    cf = View(car, "top", s2, 75, 100)          # carrier visto de frente (cara de la placa)
    cs = View(car, "side_zy", s2, 150, 100)       # carrier de lado
    av = View(arm, "front", 1.0, 260, 95)       # brazo de lado (observador en -Y)
    at = View(arm, "top", 1.0, 360, 95, hidden=False)
    for v, name in ((cf, "CARRIER (B) — FRENTE 2:1"), (cs, "CARRIER — LATERAL 2:1"),
                    (av, "BRAZO (A) — LATERAL 1:1"), (at, "BRAZO — SUPERIOR 1:1")):
        g += v.svg()
        g.append(txt((v.left_x() + v.right_x()) / 2, v.top_y() - 12, name, 3, cls="h"))
    aw, ah = p("assembly_width"), p("assembly_height")
    xo0, xo1 = aw / 2 - p("carrier_outer_width") / 2, aw / 2 + p("carrier_outer_width") / 2
    yo0, yo1 = ah / 2 - p("carrier_outer_height") / 2, ah / 2 + p("carrier_outer_height") / 2
    zt = p("carrier_ear_depth") + p("pcb_standoff_height")
    UL = (p("upper_hole_x_left"), p("upper_hole_y"), zt); UR = (p("upper_hole_x_right"), p("upper_hole_y"), zt)
    LL = (p("lower_hole_x_left"), p("lower_hole_y"), zt); LR = (p("lower_hole_x_right"), p("lower_hole_y"), zt)
    g += dim(cf.M(UL), cf.M(UR), cf.top_y() - 4, p("upper_hole_spacing"))
    g += dim(cf.M(LL), cf.M(LR), cf.bottom_y() + 6, p("lower_hole_spacing"))
    g += dim(cf.M((xo0, yo0, 0)), cf.M((xo1, yo0, 0)), cf.bottom_y() + 14, p("carrier_outer_width"))
    g += dim(cf.M((xo1, yo0, 0)), cf.M((xo1, yo1, 0)), cf.right_x() + 6, p("carrier_outer_height"), horizontal=False)
    g += dim(cf.M((xo0, 0, 0)), cf.M((xo0, p("lower_hole_y"), 0)), cf.left_x() - 5, p("lower_hole_y"), horizontal=False)
    g += dim(cf.M((xo0, 0, 0)), cf.M((xo0, p("upper_hole_y"), 0)), cf.left_x() - 13, p("upper_hole_y"), horizontal=False)
    g.append(txt(cf.left_x(), cf.bottom_y() + 24, "Origen (0,0) = esquina inferior izquierda de la placa; cotas de Y desde ahí.", 2.4, "start"))
    g += dim(cs.M((xo0, yo0, 0)), cs.M((xo0, yo0, p("carrier_ear_depth"))), cs.bottom_y() + 6, p("carrier_ear_depth"))
    g += dim(cs.M((xo0, yo1, p("carrier_ear_depth"))), cs.M((xo0, yo1, zt)), cs.top_y() - 4, p("pcb_standoff_height"))
    g.append(txt(cs.left_x(), cs.bottom_y() + 16, "cara trasera (cama)", 2.2, "start", cls="z"))
    # brazo: caída y adelanto respecto al piso
    px, pz, dft, dfx = p("pivot_x"), p("pivot_z"), p("deck_top_z"), p("deck_front_x")
    g += dim(av.M((dfx, 0, dft)), av.M((dfx, 0, pz)), av.right_x() + 8, p("drop_height"), horizontal=False,
             label="%g caída" % p("drop_height"))
    g += dim(av.M((px, 0, pz)), av.M((dfx, 0, pz)), av.bottom_y() + 8, p("forward_offset"), label="%g adelanto" % p("forward_offset"))
    g.append(txt(*av.M((dfx + 1, 0, dft + 6)), "borde del piso", 2.2, "start", cls="z"))
    mx, cy = p("mount_x_center"), p("deck_center_y")
    g += dim(at.M((mx + 6, cy - p("mount_slot_spacing") / 2, 0)), at.M((mx + 6, cy + p("mount_slot_spacing") / 2, 0)),
             at.right_x() + 6, p("mount_slot_spacing"), horizontal=False)
    g += dim(at.M((mx - p("deck_mount_width") / 2, cy - p("deck_mount_length") / 2, 0)),
             at.M((mx + p("deck_mount_width") / 2, cy - p("deck_mount_length") / 2, 0)), at.bottom_y() + 7, p("deck_mount_width"))
    rows = [("Agujero M2", "X", "Y"), ("Superior izq.", "%.2f" % UL[0], "%.2f" % UL[1]), ("Superior der.", "%.2f" % UR[0], "%.2f" % UR[1]),
            ("Inferior izq.", "%.2f" % LL[0], "%.2f" % LL[1]), ("Inferior der.", "%.2f" % LR[0], "%.2f" % LR[1])]
    tx, ty = 230, 185
    g.append(txt(tx, ty - 6, "PATRÓN M2 MEDIDO (no rectangular)", 3, "start", cls="h"))
    for i, r in enumerate(rows):
        for c, v in zip((0, 30, 48), r):
            g.append(txt(tx + c, ty + i * 5, v, 2.5, "start", cls="h" if i == 0 else "t"))
    notes = [
        "M2: paso Ø%g, tuerca M2 cautiva detrás (e/c %g x %g). Pivote: M3 Ø%g, tuerca en la cara interior de cada pata." %
        (p("m2_clearance"), p("m2_nut_af"), p("m2_nut_depth"), p("m3_clearance")),
        "Base: 2 ranuras M3 %gx%g a %g mm, sobre los puntos de expansión frontales del piso v1. Inclinación V1: %g° hacia abajo." %
        (p("mount_slot_width"), p("mount_slot_length"), p("mount_slot_spacing"), p("tilt_angle")),
        "Imprimir: brazo con la cara superior de la base sobre la cama; carrier con la cara trasera sobre la cama. Sin soportes.",
        "Todos los valores en la hoja ROVER_CAM_PARAMETERS del .FCStd.",
    ]
    g += title_block(W, H, "Soporte ESP32-CAM v1", "BL-CAM-001", "indicada", "1/1", notes, date="2026-10-15")
    App.closeDocument(doc.Name)
    print("escrito", write_sheet(os.path.join(out, "ESP32_CAM_Rover_Mount_plano"), W, H, g, "PlanoCam"))


ejecutar(main)
