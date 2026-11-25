"""Plano acotado del open frame v2 (A3): frente, trasera y lateral del frame a 2:1, brazo a 1:1.

  fc.sh run make_drawing.py -- --model out/ESP32_CAM_Open_Frame.FCStd --out out/
  fc.sh gui export_pdf.py -- --dir out
"""
import sys, os, math
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
    sh = doc.getObject("ROVER_ESP32_CAM_MOUNT")
    def p(n):
        v = sh.get(n)
        return float(v.Value) if hasattr(v, "Value") else float(v)
    W, H = 420.0, 297.0
    fr, arm = doc.getObject("OpenFrame").Shape, doc.getObject("Arm").Shape
    g = []
    ff = View(fr, "top", 2.0, 80, 95)
    fb = View(fr, "bottom", 2.0, 205, 95)
    fs = View(fr, "side_zy", 2.0, 300, 95)
    av = View(arm, "front", 1.0, 370, 95)
    for v, name in ((ff, "FRAME — FRONTAL 2:1"), (fb, "FRAME — TRASERA 2:1 (tuercas)"), (fs, "FRAME — LATERAL 2:1"),
                    (av, "BRAZO — LATERAL 1:1")):
        g += v.svg()
        g.append(txt((v.left_x() + v.right_x()) / 2, v.top_y() - 12, name, 3, cls="h"))
    top = p("frame_depth") + p("standoff_height")
    A = (p("upper_hole_x_left"), p("upper_hole_y"), top); B = (p("upper_hole_x_right"), p("upper_hole_y"), top)
    C = (p("lower_hole_x_left"), p("lower_hole_y_left"), top); D = (p("lower_hole_x_right"), p("lower_hole_y_right"), top)
    aw, ah = p("assembly_width"), p("assembly_height")
    xo0, xo1 = aw / 2 - p("frame_outer_width") / 2, aw / 2 + p("frame_outer_width") / 2
    yo0, yo1 = ah / 2 - p("frame_outer_height") / 2, ah / 2 + p("frame_outer_height") / 2
    g += dim(ff.M(A), ff.M(B), ff.top_y() - 4, p("top_hole_spacing"))
    g += dim(ff.M(C), ff.M(D), ff.bottom_y() + 6, math.hypot(D[0] - C[0], D[1] - C[1]), label="%.2f (C-D)" % p("bottom_hole_spacing"))
    g += dim(ff.M((xo0, yo0, 0)), ff.M((xo1, yo0, 0)), ff.bottom_y() + 14, p("frame_outer_width"))
    g += dim(ff.M((xo0, yo0, 0)), ff.M((xo0, yo1, 0)), ff.left_x() - 6, p("frame_outer_height"), horizontal=False)
    g += dim(ff.M((A[0], C[1], 0)), ff.M(A), ff.left_x() - 14, A[1] - C[1], horizontal=False, label="%.2f" % (A[1] - C[1]))
    g += dim(fs.M((xo0, yo0, 0)), fs.M((xo0, yo0, top)), fs.bottom_y() + 6, top, label="%.1f" % top)
    g += dim(fs.M((xo0, yo1, p("frame_depth"))), fs.M((xo0, yo1, top)), fs.top_y() - 4, p("standoff_height"))
    px, pz, dft, dfx = p("pivot_x"), p("pivot_z"), p("deck_top_z"), p("deck_front_x")
    g += dim(av.M((dfx, 0, dft)), av.M((dfx, 0, pz)), av.right_x() + 6, p("arm_height"), horizontal=False, label="%g" % p("arm_height"))
    g += dim(av.M((px, 0, pz)), av.M((dfx, 0, pz)), av.bottom_y() + 8, p("arm_offset"), label="%g" % p("arm_offset"))
    rows = [("Punto M2", "X", "Y", "Tipo"),
            ("A sup-izq", "%.2f" % A[0], "%.2f" % A[1], "agujero Ø%.1f (datum)" % p("m2_clearance")),
            ("B sup-der", "%.2f" % B[0], "%.2f" % B[1], "agujero Ø%.1f (datum)" % p("m2_clearance")),
            ("C inf-izq", "%.2f" % C[0], "%.2f" % C[1], "ranura %gx%g, +-%.2f" % (p("lower_slot_length"), p("lower_slot_width"), (p("lower_slot_length") - p("lower_slot_width")) / 2)),
            ("D inf-der", "%.2f" % D[0], "%.2f" % D[1], "ranura %gx%g, +-%.2f" % (p("lower_slot_length"), p("lower_slot_width"), (p("lower_slot_length") - p("lower_slot_width")) / 2))]
    tx, ty = 30, 205
    g.append(txt(tx, ty - 6, "PATRÓN M2 (origen: esquina inf-izq de la placa; C y D desde A-C, B-D, C-D y A-D)", 3, "start", cls="h"))
    for i, r in enumerate(rows):
        for c, v in zip((0, 26, 44, 62), r):
            g.append(txt(tx + c, ty + i * 5, v, 2.5, "start", cls="h" if i == 0 else "t"))
    notes = [
        "Placa suspendida a %.1f mm del frame: sólo 4 columnas tocan la placa; montantes fuera de su huella." % p("standoff_height"),
        "M2 con tuerca cautiva detrás (hexágono arriba, canal de %.1f abajo). Tornillo M2x16 aprox." % p("lower_nut_channel"),
        "Unión rígida al brazo: 2 M3 por lado (pivote + bloqueo a %g mm). Inclinación V1 %g° hacia abajo." % (p("lock_distance"), p("tilt_angle")),
        "Imprimir el frame plano (cara trasera en la cama) y el brazo con la base boca abajo. Sin soportes.",
    ]
    g += title_block(W, H, "Open frame ESP32-CAM v2", "BL-CAM-002", "indicada", "1/1", notes, date="2026-10-15")
    App.closeDocument(doc.Name)
    print("escrito", write_sheet(os.path.join(out, "ESP32_CAM_Open_Frame_plano"), W, H, g, "PlanoOpenFrame"))


ejecutar(main)
