"""Caja de batería y plataforma de sensores del rover (battery-box v1).

  ~/.claude/skills/freecad/scripts/fc.sh run build_box.py -- --out <carpeta>

Piezas:
  Base  caja abierta: paredes, 4 torres de esquina (mounting pad + boss de tapa),
        topes de batería, zona de cableado, anclaje de brida, salida de cable,
        ranuras de ventilación
  Lid   tapa plana con labio de alineación y nervaduras sólo sobre la zona de cables
Referencias: Battery_Ref (pack 75 x 67 x 20), Compartment_Env, Zone_LiDAR, Zone_UNOQ.

Coordenadas: origen en una esquina inferior de la caja, X a lo largo (0-128.92),
Y a lo ancho (0-76), Z=0 en la cara de apoyo. La batería ocupa el extremo X bajo y
la zona de cableado el extremo X alto, donde está la salida de cable.
"""
import sys, os, math, json
sys.path.insert(0, os.path.expanduser("~/.claude/skills/freecad/scripts"))
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "common"))
from fchelpers import *  # noqa: F401,F403
from fcparam import E, build_params, sk, feat, try_fillet, SKETCHES
import Import

SHEET = "ROVER_BATTERY_SENSOR_BOX"
NAME = "ROVER_Battery_Sensor_Enclosure"

PARAMS = [
    ("# Caja",),
    ("enclosure_length", 128.92, "mm", None, "máximo"),
    ("enclosure_width", 76.00, "mm", None, "máximo"),
    ("internal_height", 28.00, "mm", None, "piso -> borde superior de las paredes"),
    ("base_thickness", 3.00, "mm", None, ""),
    ("wall_thickness", 3.00, "mm", None, ""),
    ("lid_thickness", 3.00, "mm", None, ""),
    ("wall_top_z", None, "mm", "=base_thickness + internal_height", "apoyo de la tapa"),
    ("enclosure_total_height", None, "mm", "=wall_top_z + lid_thickness", ""),
    ("minimum_wall", 2.40, "mm", None, "6 líneas de 0.4"),
    ("fillet_radius", 2.00, "mm", None, ""),
    ("first_layer_chamfer", 0.50, "mm", None, "contra la pata de elefante"),
    ("# Torres de esquina = mounting pad + boss de tapa",),
    ("mounting_pad_size", 12.00, "mm", None, "desde la esquina exterior; sólido Z 0..lid_screw_bottom_z para perforar M3"),
    ("lid_boss_outer_diameter", 8.00, "mm", None, "inscrito en la torre"),
    ("lid_boss_center", None, "mm", "=mounting_pad_size / 2", "desde cada cara exterior"),
    ("lid_screw_clearance", 3.40, "mm", None, "tapa y piloto en la torre"),
    ("lid_insert_diameter", 4.00, "mm", None, "inserto M3 termofijado; ajustar al inserto real"),
    ("lid_insert_depth", 6.00, "mm", None, ""),
    ("lid_screw_depth", 16.00, "mm", None, "piloto Ø3.4 desde arriba"),
    ("lid_screw_bottom_z", None, "mm", "=wall_top_z - lid_screw_depth", ""),
    ("m3_nut_af", 5.90, "mm", None, "tuerca M3 5.5 + 0.4 (ranura deslizante)"),
    ("m3_nut_slot_height", 2.80, "mm", None, "tuerca 2.4 + 0.4"),
    ("m3_nut_slot_z", 19.00, "mm", None, "cara inferior de la ranura"),
    ("m3_nut_slot_reach", 3.40, "mm", None, "centro -> fondo de la ranura (radio de esquinas)"),
    ("# Batería (medida)",),
    ("battery_length", 75.00, "mm", None, "paralelo a X"),
    ("battery_width", 67.00, "mm", None, "paralelo a Y"),
    ("battery_height", 20.00, "mm", None, ""),
    ("# Compartimento y topes",),
    ("battery_compartment_length", 77.00, "mm", None, "1 mm de holgura por extremo"),
    ("battery_compartment_width", 69.00, "mm", None, "1 mm de holgura por lado"),
    ("battery_compartment_height", 23.00, "mm", None, "3 mm sobre el pack"),
    ("battery_compartment_x0", None, "mm", "=mounting_pad_size", "el pack apoya en las torres del extremo X bajo"),
    ("battery_compartment_x1", None, "mm", "=battery_compartment_x0 + battery_compartment_length", ""),
    ("battery_compartment_y0", None, "mm", "=(enclosure_width - battery_compartment_width) / 2", ""),
    ("battery_stop_thickness", 3.00, "mm", None, "topes longitudinales (lado cables)"),
    ("battery_stop_height", 10.00, "mm", None, "editable 8-12"),
    ("battery_stop_length", 15.00, "mm", None, "largo en Y de cada tope longitudinal; el hueco central deja pasar cables"),
    ("battery_lateral_stop_thickness", None, "mm", "=(enclosure_width - 2 * wall_thickness - battery_compartment_width) / 2",
     "lo que cabe entre pared y compartimento"),
    ("battery_lateral_stop_length", 30.00, "mm", None, ""),
    ("stop_fillet", 1.00, "mm", None, "aristas de los topes hacia el pack"),
    ("# Zona de cableado",),
    ("cable_management_x0", None, "mm", "=battery_compartment_x1 + battery_stop_thickness", ""),
    ("cable_management_length", None, "mm", "=enclosure_length - wall_thickness - cable_management_x0",
     "objetivo del encargo 45; esto es lo que cabe (ver README)"),
    ("cable_exit_width", 8.00, "mm", None, "2 x 22 AWG"),
    ("cable_exit_height", 5.00, "mm", None, ""),
    ("cable_exit_radius", 1.50, "mm", None, ""),
    ("cable_exit_z", 7.00, "mm", None, "borde inferior de la salida, alineado con el anclaje"),
    ("cable_exit_y", None, "mm", "=enclosure_width / 2", "salida en la pared del extremo X alto"),
    ("tie_slot_width", 3.00, "mm", None, "alto del túnel de la brida"),
    ("tie_slot_length", 10.00, "mm", None, "ancho del túnel (en la dirección del cable)"),
    ("tie_anchor_x", 112.00, "mm", None, "centro del anclaje"),
    ("tie_anchor_width", 12.00, "mm", None, "en Y; la brida lo atraviesa en Y"),
    ("tie_anchor_roof", 2.00, "mm", None, "techo sobre el túnel; arriba apoyan los cables"),
    ("# Tapa",),
    ("lid_lip_height", 2.00, "mm", None, ""),
    ("lid_lip_clearance", 0.30, "mm", None, "por lado"),
    ("lid_lip_thickness", 2.40, "mm", None, ""),
    ("lid_lip_inner", None, "mm", "=wall_thickness + lid_lip_clearance + lid_lip_thickness", "cara interior del labio"),
    ("lid_rib_thickness", 3.00, "mm", None, ""),
    ("lid_rib_height", 3.00, "mm", None, ""),
    ("lid_rib_x1", None, "mm", "=cable_management_x0 + lid_rib_thickness / 2 + 1 mm", "primera nervadura transversal"),
    ("lid_rib_x2", None, "mm", "=(lid_rib_x1 + enclosure_length - lid_lip_inner) / 2", ""),
    ("lid_rib_y1", None, "mm", "=enclosure_width / 3", "nervaduras longitudinales"),
    ("lid_rib_y2", None, "mm", "=enclosure_width * 2 / 3", ""),
    ("minimum_vertical_clearance", 2.00, "mm", None, ""),
    ("# Ventilación (ranuras horizontales en las paredes largas)",),
    ("vent_length", 18.00, "mm", None, ""),
    ("vent_height", 2.00, "mm", None, ""),
    ("vent_z1", 17.00, "mm", None, "centros; por encima de los topes"),
    ("vent_z2", 22.00, "mm", None, ""),
    ("vent_x1", None, "mm", "=battery_compartment_x0 + battery_compartment_length / 4", ""),
    ("vent_x2", None, "mm", "=battery_compartment_x0 + battery_compartment_length * 3 / 4", ""),
    ("vent_x3", None, "mm", "=(cable_management_x0 + enclosure_length - mounting_pad_size) / 2", ""),
    ("# Zonas conceptuales sobre la tapa (sin agujeros)",),
    ("lidar_zone_size", 50.00, "mm", None, ""),
    ("lidar_zone_x0", None, "mm", "=enclosure_length - wall_thickness - lidar_zone_size", "extremo de la zona de cables"),
]


def build_base(doc, P):
    body = doc.addObject("PartDesign::Body", "Base")
    L, W, t = P["enclosure_length"], P["enclosure_width"], P["wall_thickness"]
    z0, zt = E.k(0), P["wall_top_z"]
    s = sk(body, "Sk_Outline")
    s.rect(z0, z0, L, W, P["fillet_radius"])
    feat(body, "Pad", "Shell", s, Length=zt)
    s = sk(body, "Sk_Cavity", offset=zt)
    s.rect(t, t, L - t, W - t)
    feat(body, "Pocket", "Cavity", s, Length=P["internal_height"])

    # torres de esquina: rectángulo desde la cara interior de las paredes, esquina interior redondeada
    a, r = P["mounting_pad_size"], P["fillet_radius"]
    s = sk(body, "Sk_Towers", offset=P["base_thickness"])
    s.poly([(t, t), (a, t), (a, a), (t, a)], [None, None, r, None])
    s.poly([(L - a, t), (L - t, t), (L - t, a), (L - a, a)], [None, None, None, r])
    s.poly([(L - a, W - a), (L - t, W - a), (L - t, W - t), (L - a, W - t)], [r, None, None, None])
    s.poly([(t, W - a), (a, W - a), (a, W - t), (t, W - t)], [None, r, None, None])
    feat(body, "Pad", "CornerTowers", s, Length=P["internal_height"])

    # topes longitudinales (lado cables) y laterales
    x1, st, sl = P["battery_compartment_x1"], P["battery_stop_thickness"], P["battery_stop_length"]
    s = sk(body, "Sk_Stops", offset=P["base_thickness"])
    s.rect(x1, t, x1 + st, t + sl)
    s.rect(x1, W - t - sl, x1 + st, W - t)
    cx = P["battery_compartment_x0"] + P["battery_compartment_length"] / 2
    ll, lt = P["battery_lateral_stop_length"], P["battery_lateral_stop_thickness"]
    s.rect(cx - ll / 2, t, cx + ll / 2, t + lt)
    s.rect(cx - ll / 2, W - t - lt, cx + ll / 2, W - t)
    feat(body, "Pad", "BatteryStops", s, Length=P["battery_stop_height"])

    # anclaje de brida: puente con túnel en Y, los cables pasan por encima hacia la salida
    aw = P["tie_slot_length"] + P["minimum_wall"] * 2
    s = sk(body, "Sk_TieAnchor", offset=P["base_thickness"])
    s.rect_c(P["tie_anchor_x"], W / 2, aw, P["tie_anchor_width"], E.k(1.0))
    feat(body, "Pad", "TieAnchor", s, Length=P["tie_slot_width"] + P["tie_anchor_roof"])
    s = sk(body, "Sk_TieTunnel", plane="XZ_Plane", offset=-(W / 2))
    s.rect(P["tie_anchor_x"] - P["tie_slot_length"] / 2, P["base_thickness"],
           P["tie_anchor_x"] + P["tie_slot_length"] / 2, P["base_thickness"] + P["tie_slot_width"])
    feat(body, "Pocket", "TieTunnel", s, Length=P["tie_anchor_width"] + 2, SideType="Symmetric")

    # salida de cable en la pared del extremo X alto
    s = sk(body, "Sk_CableExit", plane="YZ_Plane", offset=L)
    s.rect_c(P["cable_exit_y"], P["cable_exit_z"] + P["cable_exit_height"] / 2,
             P["cable_exit_width"], P["cable_exit_height"], P["cable_exit_radius"])
    feat(body, "Pocket", "CableExit", s, Length=t + 0.5)

    # ventilación: 2 ranuras x 3 grupos en cada pared larga
    vh = P["vent_height"]
    s = sk(body, "Sk_Vents", plane="XZ_Plane", offset=-(W / 2))
    for vx in ("vent_x1", "vent_x2", "vent_x3"):
        for vz in ("vent_z1", "vent_z2"):
            s.rect_c(P[vx], P[vz], P["vent_length"], vh, vh / 2 - 0.05)
    feat(body, "Pocket", "Vents", s, Type="ThroughAll", SideType="Symmetric")

    # bosses de tapa: inserto arriba, piloto Ø3.4, ranura de tuerca cautiva abierta hacia dentro
    c = P["lid_boss_center"]
    corners = [(c, c, 1), (L - c, c, -1), (L - c, W - c, -1), (c, W - c, 1)]
    s = sk(body, "Sk_Inserts", offset=zt)
    for x, y, _ in corners:
        s.circle(x, y, P["lid_insert_diameter"] / 2)
    feat(body, "Pocket", "InsertHoles", s, Length=P["lid_insert_depth"])
    s = sk(body, "Sk_ScrewPilots", offset=zt)
    for x, y, _ in corners:
        s.circle(x, y, P["lid_screw_clearance"] / 2)
    feat(body, "Pocket", "ScrewPilots", s, Length=P["lid_screw_depth"])
    af, rr = P["m3_nut_af"], P["m3_nut_slot_reach"]
    s = sk(body, "Sk_NutSlots", offset=P["m3_nut_slot_z"] + P["m3_nut_slot_height"])
    for x, y, d in corners:
        if d > 0:
            s.rect(x - rr, y - af / 2, a, y + af / 2)
        else:
            s.rect(L - a, y - af / 2, x + rr, y + af / 2)
    feat(body, "Pocket", "NutSlots", s, Length=P["m3_nut_slot_height"])
    doc.recompute()

    # fillets por geometría (try_fillet descarta los que dejan el sólido inválido)
    v = {k: P[k].v for k in P}
    log = {}
    x1 = v["battery_compartment_x1"]
    ztop = v["base_thickness"] + v["battery_stop_height"]
    def stop_pack_edges(e):
        # aristas convexas de los topes longitudinales hacia el pack: la superior de la cara X=x1
        # y las dos verticales libres de esa cara
        bb = e.BoundBox
        on_face = abs(bb.XMin - x1) < 1e-6 and abs(bb.XMax - x1) < 1e-6
        top = abs(bb.ZMin - ztop) < 1e-6 and abs(bb.ZMax - ztop) < 1e-6
        vert = (es_vertical(e) and bb.ZMin > v["base_thickness"] - 1e-6 and abs(bb.ZMax - ztop) < 1e-6
                and v["wall_thickness"] + 1 < bb.YMin < v["enclosure_width"] - v["wall_thickness"] - 1)
        return on_face and (top or vert)
    for r_ in (v["stop_fillet"], 0.6):
        f, n = try_fillet(doc, body, "StopFillets", stop_pack_edges, r_)
        if f is not None:
            break
    log["topes"] = (f is not None, n, r_)
    def bottom_outer(e):
        bb = e.BoundBox
        return bb.ZMax < 1e-6
    f, n = try_fillet(doc, body, "FirstLayerChamfer", bottom_outer, v["first_layer_chamfer"], kind="Chamfer")
    log["chaflán primera capa"] = (f is not None, n)
    return body, log


def build_lid(doc, P):
    body = doc.addObject("PartDesign::Body", "Lid")
    L, W, t = P["enclosure_length"], P["enclosure_width"], P["wall_thickness"]
    z0, zt = E.k(0), P["wall_top_z"]
    s = sk(body, "Sk_LidPlate", offset=zt)
    s.rect(z0, z0, L, W, P["fillet_radius"])
    feat(body, "Pad", "LidPlate", s, Length=P["lid_thickness"])

    # labio: 4 tramos, interrumpidos en las torres de esquina
    a, cl, lt = P["mounting_pad_size"], P["lid_lip_clearance"], P["lid_lip_thickness"]
    o = t + cl
    s = sk(body, "Sk_Lip", offset=zt)
    s.rect(a + cl, o, L - a - cl, o + lt)
    s.rect(a + cl, W - o - lt, L - a - cl, W - o)
    s.rect(o, a + cl, o + lt, W - a - cl)
    s.rect(L - o - lt, a + cl, L - o, W - a - cl)
    feat(body, "Pad", "Lip", s, Length=P["lid_lip_height"], Reversed=True)

    # nervaduras sólo sobre la zona de cables (nada rígido sobre el pack)
    li, rt = P["lid_lip_inner"], P["lid_rib_thickness"]
    s = sk(body, "Sk_Ribs", offset=zt)
    for xr in ("lid_rib_x1", "lid_rib_x2"):
        s.rect(P[xr] - rt / 2, li, P[xr] + rt / 2, W - li)
    for yr in ("lid_rib_y1", "lid_rib_y2"):
        s.rect(P["lid_rib_x1"] + rt / 2, P[yr] - rt / 2, L - li, P[yr] + rt / 2)
    feat(body, "Pad", "Ribs", s, Length=P["lid_rib_height"], Reversed=True)

    c = P["lid_boss_center"]
    s = sk(body, "Sk_LidHoles", offset=zt + P["lid_thickness"])
    for x, y in ((c, c), (L - c, c), (L - c, W - c), (c, W - c)):
        s.circle(x, y, P["lid_screw_clearance"] / 2)
    feat(body, "Pocket", "LidHoles", s, Type="ThroughAll")
    doc.recompute()
    v = {k: P[k].v for k in P}
    zt_ = v["wall_top_z"] + v["lid_thickness"]
    f, n = try_fillet(doc, body, "TopChamfer", lambda e: e.BoundBox.ZMin > zt_ - 1e-6 and e.Curve.TypeId != "Part::GeomCircle",
                      v["first_layer_chamfer"], kind="Chamfer")
    return body, {"chaflán cara superior (cama)": (f is not None, n)}


def overhangs(shape):
    """Caras que miran hacia abajo (>44° desde la vertical) que no apoyan en la cama:
    devuelve el menor lado de cada una (≈ luz del puente)."""
    z0 = shape.optimalBoundingBox().ZMin
    bad = []
    for fc in shape.Faces:
        pr = fc.ParameterRange
        try:
            n = fc.normalAt((pr[0] + pr[1]) / 2, (pr[2] + pr[3]) / 2)
        except Exception:
            continue
        bb = fc.optimalBoundingBox()
        if n.z < -0.72 and fc.Area > 1 and abs(bb.ZMax - z0) > 1e-3:
            bad.append(round(min(bb.XLength, bb.YLength), 1))
    return bad


def ref_box(doc, name, l, w, h, pos):
    o = doc.addObject("Part::Feature", name)
    o.Shape = Part.makeBox(l, w, h, pos)
    return o


def main():
    args = argv_script()
    out = os.path.abspath(args[args.index("--out") + 1])
    os.makedirs(out, exist_ok=True)
    doc = App.newDocument(NAME)
    P, v = build_params(doc, SHEET, PARAMS)
    base, flog = build_base(doc, P)
    lid, llog = build_lid(doc, P)
    flog.update(llog)

    L, W, t, bt = v["enclosure_length"], v["enclosure_width"], v["wall_thickness"], v["base_thickness"]
    zt = v["wall_top_z"]
    bx0 = v["battery_compartment_x0"] + (v["battery_compartment_length"] - v["battery_length"]) / 2
    by0 = (W - v["battery_width"]) / 2
    bat = ref_box(doc, "Battery_Ref", v["battery_length"], v["battery_width"], v["battery_height"], V(bx0, by0, bt))
    comp = ref_box(doc, "Compartment_Env", v["battery_compartment_length"], v["battery_compartment_width"],
                   v["battery_compartment_height"], V(v["battery_compartment_x0"], v["battery_compartment_y0"], bt))
    ztop = zt + v["lid_thickness"]
    lz = ref_box(doc, "Zone_LiDAR", v["lidar_zone_size"], v["lidar_zone_size"], 0.2,
                 V(v["lidar_zone_x0"], W / 2 - v["lidar_zone_size"] / 2, ztop))
    uq = ref_box(doc, "Zone_UNOQ", v["lidar_zone_x0"] - 2 - t, W - 2 * t, 0.2, V(t, t, ztop))
    verificar(doc, base)
    verificar(doc, lid)

    B, Lh, BAT = base.Shape, lid.Shape, bat.Shape
    R = []
    def chk(n, ok, d):
        R.append((n, "OK" if ok is True else ("FALLA" if ok is False else ok), d))
    asm = B.fuse(Lh)
    bb = asm.optimalBoundingBox()
    chk("1. Caja exterior", abs(bb.XLength - L) < 0.01 and abs(bb.YLength - W) < 0.01,
        "%.2f x %.2f x %.2f mm (alto total)" % (bb.XLength, bb.YLength, bb.ZLength))
    bbb = BAT.BoundBox
    chk("2. Batería", (round(bbb.XLength, 2), round(bbb.YLength, 2), round(bbb.ZLength, 2)) == (75, 67, 20),
        "%.2f x %.2f x %.2f" % (bbb.XLength, bbb.YLength, bbb.ZLength))
    chk("3. Orientación", bbb.XLength > bbb.YLength, "75 mm en X (largo de la caja), 67 mm en Y; sin rotar")
    # compartimento medido en el sólido: rayos desde el centro del pack hacia cada lado, a media altura de los topes
    zc = bt + v["battery_stop_height"] / 2
    ctr = V(bx0 + v["battery_length"] / 2, W / 2, zc)
    def ray(d, y=None):
        p0 = V(ctr.x, ctr.y if y is None else y, zc)
        ln = Part.LineSegment(p0, p0 + d * 200).toShape()
        hits = [p for p in (ln.common(B).Vertexes)]
        dist = [(h.Point - p0).Length for h in hits]
        return min(dist) if dist else None
    yc_stop = t + 3.0  # tramo plano de la torre, antes del redondeo interior
    xm = ray(V(-1, 0, 0), yc_stop) + ray(V(1, 0, 0), yc_stop)
    ym = ray(V(0, -1, 0)) + ray(V(0, 1, 0))
    chk("4. Compartimento", abs(xm - v["battery_compartment_length"]) < 0.01 and abs(ym - v["battery_compartment_width"]) < 0.01,
        "medido en el sólido %.2f x %.2f (torres <-> topes longitudinales, topes laterales)" % (xm, ym))
    above = Part.makeBox(v["battery_length"], v["battery_width"], 100, V(bx0, by0, bt))
    chk("5. Batería desmontable", B.common(above).Volume < 1e-6,
        "sin tapa, nada de la base sobre la huella del pack (%.4f mm3): sale en vertical" % B.common(above).Volume)
    above_floor = B.common(Part.makeBox(L, W, 100, V(0, 0, bt + 0.01)))
    gap = above_floor.distToShape(BAT)[0]
    chk("6. Topes sin compresión", B.common(BAT).Volume < 1e-6 and gap >= 0.99,
        "interferencia 0; apoya sólo en el fondo; holgura mínima pack-paredes/topes %.2f mm" % gap)
    vg = Lh.distToShape(BAT)[0]
    over = Lh.common(Part.makeBox(v["battery_length"], v["battery_width"], zt - bt - v["battery_height"],
                                  V(bx0, by0, bt + v["battery_height"])))
    chk("7. Holgura vertical >= %.0f mm" % v["minimum_vertical_clearance"], vg >= v["minimum_vertical_clearance"],
        "tapa-pack %.2f mm (labio); techo de la tapa a %.2f mm del pack" % (vg, zt - bt - v["battery_height"]))
    cz = Part.makeBox(v["cable_management_length"], W - 2 * t, v["internal_height"], V(v["cable_management_x0"], t, bt))
    free = cz.cut(B).cut(Lh).Volume
    chk("8. Zona de cableado", "AVISO" if v["cable_management_length"] < 45 else True,
        "%.2f x %.2f x %.2f mm, %.1f cm3 libres (objetivo 45 mm de largo: no cabe con compartimento 77 + tope 3 + torres %g)"
        % (v["cable_management_length"], W - 2 * t, v["internal_height"], free / 1000, v["mounting_pad_size"]))
    od = 2.0
    ew, eh = v["cable_exit_width"], v["cable_exit_height"]
    p_in = V(L - t / 2, v["cable_exit_y"], v["cable_exit_z"] + eh / 2)
    chk("9. Salida 22 AWG", not B.isInside(p_in, 1e-6, True) and 2 * od + 1 <= ew and od + 1 <= eh,
        "ventana %gx%g r%g abierta en la pared X=%.2f; 2 cables de Ø%.1f (22 AWG con aislante grueso) lado a lado dejan %.1f x %.1f mm"
        % (ew, eh, v["cable_exit_radius"], L, od, ew - 2 * od, eh - od))
    tun = V(v["tie_anchor_x"], W / 2, bt + v["tie_slot_width"] / 2)
    roof = v["base_thickness"] + v["tie_slot_width"] + v["tie_anchor_roof"]
    chk("10. Strain relief", not B.isInside(tun, 1e-6, True) and B.isInside(V(v["tie_anchor_x"], W / 2, roof - 0.5), 1e-6, True),
        "túnel %gx%g (brida de hasta ~9 mm) bajo un puente; cables apoyados a Z=%.1f, salida desde Z=%.1f; anclaje a %.1f mm de la salida"
        % (v["tie_slot_length"], v["tie_slot_width"], roof, v["cable_exit_z"],
           L - t - (v["tie_anchor_x"] + v["tie_slot_length"] / 2 + v["minimum_wall"])))
    lift = max(B.common(Lh.translated(V(0, 0, dz))).Volume for dz in (0, 0.5, 1, 2, 3))
    lg = B.distToShape(Lh.cut(Part.makeBox(L, W, 10, V(0, 0, zt))))[0] if Lh.cut(Part.makeBox(L, W, 10, V(0, 0, zt))).Volume > 0 else -1
    chk("11. Tapa desmontable", lift < 1e-6,
        "sin interferencia al subirla 0-3 mm; labio a %.2f mm de las paredes y torres; 4 M3" % lg)
    tw = B.common(Part.makeCompound([Part.makeBox(v["mounting_pad_size"], v["mounting_pad_size"], 40, V(x, y, -1))
                                     for x in (0, L - v["mounting_pad_size"]) for y in (0, W - v["mounting_pad_size"])]))
    chk("12. Bosses vs batería", tw.distToShape(BAT)[0] >= 0.99,
        "torres a %.2f mm del pack; boss Ø%g inscrito en torre de %g" % (tw.distToShape(BAT)[0], v["lid_boss_outer_diameter"], v["mounting_pad_size"]))
    ribs_only = doc.getObject("Ribs").AddSubShape
    rv = ribs_only.common(Part.makeBox(v["battery_length"], v["battery_width"], 100, V(bx0, by0, 0))).Volume
    chk("13. Nervaduras vs batería", rv < 1e-6 and ribs_only.distToShape(BAT)[0] >= v["minimum_vertical_clearance"],
        "ninguna nervadura sobre la huella del pack (%.4f mm3); distancia %.2f mm" % (rv, ribs_only.distToShape(BAT)[0]))
    top = [f for f in Lh.Faces if f.Surface.TypeId == "Part::GeomPlane" and abs(f.CenterOfMass.z - ztop) < 1e-6]
    chk("14. Tapa plana (UNO Q)", len(top) == 1, "%d cara superior plana, %.0f mm2 (sólo 4 agujeros M3 de esquina)"
        % (len(top), sum(f.Area for f in top)))
    lzbox = Part.makeBox(v["lidar_zone_size"], v["lidar_zone_size"], v["lid_thickness"],
                         V(v["lidar_zone_x0"], W / 2 - v["lidar_zone_size"] / 2, zt))
    filled = Lh.common(lzbox).Volume / lzbox.Volume
    chk("15. Zona LiDAR", filled > 0.999, "50x50 en X %.2f-%.2f, maciza al %.2f%% (sin agujeros); debajo hay nervaduras: perforar entre ellas"
        % (v["lidar_zone_x0"], v["lidar_zone_x0"] + 50, filled * 100))
    walls = {"pared": t, "fondo": bt, "tapa": v["lid_thickness"], "labio": v["lid_lip_thickness"],
             "nervadura": v["lid_rib_thickness"], "tope longitudinal": v["battery_stop_thickness"],
             "techo del anclaje": v["tie_anchor_roof"],
             "torre: tuerca -> cara exterior": v["lid_boss_center"] - v["m3_nut_slot_reach"],
             "torre: tuerca -> cara interior": v["mounting_pad_size"] - v["lid_boss_center"] - v["m3_nut_af"] / 2,
             "torre: inserto -> cara exterior": v["lid_boss_center"] - v["lid_insert_diameter"] / 2,
             "tope lateral (se suma a la pared)": v["battery_lateral_stop_thickness"]}
    low = {k: w for k, w in walls.items() if w < v["minimum_wall"] and "lateral" not in k and "anclaje" not in k}
    chk("16. Paredes imprimibles", "AVISO" if low else True,
        "%s; bajo %.1f sólo el tope lateral (0.5, va pegado a la pared) y el techo del anclaje (2.0, 5 capas)%s"
        % ({k: round(w, 2) for k, w in walls.items()}, v["minimum_wall"], ("; REVISAR %s" % low) if low else ""))
    lid_p = Lh.copy(); lid_p.rotate(V(0, 0, 0), V(1, 0, 0), 180); lid_p.translate(V(0, 0, -lid_p.optimalBoundingBox().ZMin))
    lid_p.translate(V(0, W, 0))
    ob, ol = overhangs(B), overhangs(lid_p)
    chk("17. Sin soportes", max(ob + ol + [0]) <= 20,
        "base fondo abajo: puentes de %s mm (ventilación, salida, túnel, tuercas); tapa boca abajo: %s"
        % (sorted(set(ob)) or "ninguno", sorted(set(ol)) or "ninguno"))
    sk_bad = [o.o.Name for o in SKETCHES if not o.o.FullyConstrained or o.o.ConflictingConstraints or o.o.RedundantConstraints]
    nexpr = sum(len(o.ExpressionEngine) for o in doc.Objects if hasattr(o, "ExpressionEngine"))
    chk("18. Paramétrico", not sk_bad, "%d sketches totalmente restringidos%s; %d expresiones atadas a %s"
        % (len(SKETCHES), "" if not sk_bad else ", SALVO " + ", ".join(sk_bad), nexpr, SHEET))

    print("\n== validaciones ==")
    for n_, st, d_ in R:
        print("[%s] %s: %s" % (st, n_, d_))
    print("fillets/chaflanes:", flog)
    for n_, s_ in (("base", B), ("tapa", Lh)):
        b_ = s_.optimalBoundingBox()
        print("%-5s %.2f x %.2f x %.2f mm, %.2f cm3, ~%.0f g PLA macizo, válido %s, sólidos %d"
              % (n_, b_.XLength, b_.YLength, b_.ZLength, s_.Volume / 1000, masa(s_), s_.isValid(), len(s_.Solids)))

    for fn, shp in (("ROVER_Battery_Box_Base.stl", B), ("ROVER_Battery_Box_Lid.stl", lid_p)):
        m, probs = validar_para_impresion(shp)
        if probs:
            raise RuntimeError("%s: %s" % (fn, probs))
        m.write(os.path.join(out, fn))
    tmp = []
    for nm, s_ in (("Base", B), ("Lid", Lh)):
        o = doc.addObject("Part::Feature", "EXP_" + nm); o.Shape = s_; o.Label = nm; tmp.append(o)
    Import.export(tmp, os.path.join(out, NAME + ".step"))
    for o in tmp:
        doc.removeObject(o.Name)
    for b in (base, lid):
        for o in b.Group:
            o.Visibility = (o == b.Tip)
    comp.Visibility = False
    doc.recompute()
    doc.saveAs(os.path.join(out, NAME + ".FCStd"))
    with open(os.path.join(out, "validation.json"), "w") as fh:
        json.dump({"checks": R, "fillets": {k: list(x) for k, x in flog.items()}}, fh, indent=1, ensure_ascii=False)
    print("escrito en", out)


ejecutar(main)
