"""Plano acotado de la caja de batería (A3, 1:1): base superior, frontal, lateral, sección y tapa inferior.

  fc.sh run make_drawing.py -- --model out/ROVER_Battery_Sensor_Enclosure.FCStd --out out/
  fc.sh gui export_pdf.py -- --dir out
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
    sh = doc.getObject("ROVER_BATTERY_SENSOR_BOX")
    def p(n):
        v = sh.get(n)
        return float(v.Value) if hasattr(v, "Value") else float(v)
    W_, H_ = 420.0, 297.0
    B, Lh = doc.getObject("Base").Shape, doc.getObject("Lid").Shape
    L, W, t, bt, zt = p("enclosure_length"), p("enclosure_width"), p("wall_thickness"), p("base_thickness"), p("wall_top_z")
    ztop = p("enclosure_total_height")
    asm = B.fuse(Lh)
    sec = B.common(Part.makeBox(L + 2, W / 2 + 1, 50, V(-1, W / 2, -1)))   # mitad Y>W/2, vista desde -Y
    g = []
    top = View(B, "top", 1.0, 100, 85)
    fr = View(asm, "front", 1.0, 100, 175)
    rt = View(asm, "right", 1.0, 215, 175)
    sc = View(sec, "front", 1.0, 100, 245, hidden=False)
    lb = View(Lh, "bottom", 1.0, 300, 85)
    for v, name in ((top, "BASE — VISTA SUPERIOR"), (fr, "FRONTAL (desde -Y)"), (rt, "LATERAL (desde +X): salida"),
                    (sc, "SECCIÓN A-A (Y = %.0f)" % (W / 2)), (lb, "TAPA — VISTA INFERIOR")):
        g += v.svg()
        g.append(txt((v.left_x() + v.right_x()) / 2, v.top_y() - 10, name, 3, cls="h"))
    # base superior
    g += dim(top.M((0, W, 0)), top.M((L, W, 0)), top.top_y() - 4, L, label="%.2f" % L)
    g += dim(top.M((0, 0, 0)), top.M((0, W, 0)), top.left_x() - 6, W, horizontal=False, label="%.2f" % W)
    x0, x1, cz = p("battery_compartment_x0"), p("battery_compartment_x1"), p("cable_management_x0")
    g += dim(top.M((x0, 0, 0)), top.M((x1, 0, 0)), top.bottom_y() + 6, x1 - x0, label="%g compartimento" % (x1 - x0))
    g += dim(top.M((cz, 0, 0)), top.M((L - t, 0, 0)), top.bottom_y() + 6, L - t - cz, label="%.2f cables" % (L - t - cz))
    g += dim(top.M((0, 0, 0)), top.M((x0, 0, 0)), top.bottom_y() + 13, x0, label="%g torre" % x0)
    y0 = p("battery_compartment_y0")
    g += dim(top.M((x0 + 30, y0, 0)), top.M((x0 + 30, W - y0, 0)), top.right_x() + 6, W - 2 * y0, horizontal=False,
             label="%g" % (W - 2 * y0))
    c = p("lid_boss_center")
    g += dim(top.M((0, c, 0)), top.M((c, c, 0)), top.bottom_y() + 20, c, label="%g" % c)
    g.append(txt(top.M((x0 + 38, W / 2, 0))[0], top.M((0, W / 2, 0))[1], "PACK 75 x 67 x 20", 3, cls="z"))
    # frontal y lateral
    g += dim(fr.M((L, 0, 0)), fr.M((L, 0, ztop)), fr.right_x() + 6, ztop, horizontal=False, label="%g" % ztop)
    g += dim(fr.M((0, 0, 0)), fr.M((0, 0, zt)), fr.left_x() - 6, zt, horizontal=False, label="%g" % zt)
    g += dim(fr.M((p("vent_x1") - p("vent_length") / 2, 0, 0)), fr.M((p("vent_x1") + p("vent_length") / 2, 0, 0)),
             fr.bottom_y() + 6, p("vent_length"), label="%g x %g" % (p("vent_length"), p("vent_height")))
    ey, ez, ew, eh = p("cable_exit_y"), p("cable_exit_z"), p("cable_exit_width"), p("cable_exit_height")
    g += dim(rt.M((L, ey - ew / 2, ez)), rt.M((L, ey + ew / 2, ez)), rt.bottom_y() + 6, ew, label="%g (r%g)" % (ew, p("cable_exit_radius")))
    g += dim(rt.M((L, ey + ew / 2, ez)), rt.M((L, ey + ew / 2, ez + eh)), rt.right_x() + 6, eh, horizontal=False, label="%g" % eh)
    g += dim(rt.M((L, 0, 0)), rt.M((L, 0, ez)), rt.left_x() - 6, ez, horizontal=False, label="%g" % ez)
    # sección
    g += dim(sc.M((x0, 0, bt + p("battery_height"))), sc.M((x0, 0, zt)), sc.left_x() - 6, zt - bt - p("battery_height"),
             horizontal=False, label="%g hasta la tapa" % (zt - bt - p("battery_height")))
    g += dim(sc.M((x1, 0, bt)), sc.M((x1, 0, bt + p("battery_stop_height"))), sc.right_x() + 6, p("battery_stop_height"),
             horizontal=False, label="tope %g" % p("battery_stop_height"))
    ax_ = p("tie_anchor_x")
    g += dim(sc.M((ax_ - p("tie_slot_length") / 2, 0, bt)), sc.M((ax_ + p("tie_slot_length") / 2, 0, bt)), sc.bottom_y() + 6,
             p("tie_slot_length"), label="túnel brida %g x %g" % (p("tie_slot_length"), p("tie_slot_width")))
    # tapa
    g += dim(lb.M((p("lid_rib_x1"), 0, zt)), lb.M((L, 0, zt)), lb.bottom_y() + 6, L - p("lid_rib_x1"),
             label="%.1f nervaduras" % (L - p("lid_rib_x1")))
    g.append(txt(lb.M((40, W / 2, zt))[0], lb.M((40, W / 2, zt))[1], "LISA: sin nada sobre el pack", 3, cls="z"))
    notes = [
        "Torres de esquina %g x %g: inserto M3 Ø%g x %g arriba, piloto Ø%g, ranura de tuerca M3 a Z=%g." % (
            p("mounting_pad_size"), p("mounting_pad_size"), p("lid_insert_diameter"), p("lid_insert_depth"),
            p("lid_screw_clearance"), p("m3_nut_slot_z")),
        "Sólido Z 0-%g bajo cada piloto: perforar ahí la fijación M3 al deck (desde abajo)." % p("lid_screw_bottom_z"),
        "Zonas en la tapa sin agujeros: LiDAR 50x50 en X %.1f-%.1f; UNO Q en el resto. Perforar al montar." % (
            p("lidar_zone_x0"), p("lidar_zone_x0") + 50),
        "Imprimir base fondo abajo y tapa boca abajo, sin soportes. PLA, 4 paredes, 5/5 capas, 30-40 %.",
    ]
    g += title_block(W_, H_, "Caja batería + plataforma sensores v1", "BL-BOX-001", "1:1", "1/1", notes,
                     notes_xy=(240, 200), date="2026-10-16")
    App.closeDocument(doc.Name)
    print("escrito", write_sheet(os.path.join(out, "ROVER_Battery_Sensor_Enclosure_plano"), W_, H_, g, "PlanoCajaBateria"))


ejecutar(main)
