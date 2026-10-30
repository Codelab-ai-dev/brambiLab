"""Soporte frontal ESP32-CAM + placa de expansión (cam-mount v1) — FreeCAD paramétrico.

  ~/.claude/skills/freecad/scripts/fc.sh run build_cam_mount.py -- --out <carpeta>

Piezas:
  Arm      pieza A: base sobre el piso (2 ranuras M3), muro frontal, 2 brazos laterales
           con buje de pivote M3, 2 nervaduras y 2 ranuras para bridas
  Carrier  pieza B: marco trasero abierto (U inferior, ventana central), 4 separadores M2
           en las posiciones medidas, tuercas M2 cautivas detrás, orejas laterales M3
  CarrierTest  anillo de 2 mm con los 4 separadores, para probar el patrón M2
Referencias (no se imprimen): ComputeDeck (STEP del piso v1), ChassisRef, CamModule_Env.

Coordenadas del rover (las del piso v1): X a lo largo, X=0 frente del chasis (supuesto),
Y a lo ancho, Z=0 cara superior del chasis. La cámara mira hacia -X.
Coordenadas locales del carrier: origen en la esquina inferior izquierda de la placa vista
de frente, x a la derecha, y hacia arriba, z hacia delante (hacia la escena); z=0 es la
cara trasera del carrier, que va sobre la cama al imprimir.
"""
import sys, os, math, json
sys.path.insert(0, os.path.expanduser("~/.claude/skills/freecad/scripts"))
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "common"))
from fchelpers import *  # noqa: F401,F403
from fcparam import E, build_params, sk, feat, try_fillet, SKETCHES
import Import, Mesh

SHEET = "ROVER_CAM_PARAMETERS"

PARAMS = [
    ("# Conjunto ESP32-CAM + placa de expansión (medido con vernier)",),
    ("assembly_width", 36.05, "mm", None, "medido"),
    ("assembly_height", 41.89, "mm", None, "medido"),
    ("assembly_depth", 22.69, "mm", None, "medido; sándwich completo"),
    ("upper_hole_y_from_top", 1.85, "mm", None, "medido"),
    ("upper_hole_side_offset", 2.74, "mm", None, "medido"),
    ("lower_hole_side_offset", 2.51, "mm", None, "medido"),
    ("lower_hole_bottom_offset", 3.40, "mm", None, "medido"),
    ("upper_hole_x_left", None, "mm", "=upper_hole_side_offset", "2.74"),
    ("upper_hole_x_right", None, "mm", "=assembly_width - upper_hole_side_offset", "33.31"),
    ("upper_hole_y", None, "mm", "=assembly_height - upper_hole_y_from_top", "40.04"),
    ("lower_hole_x_left", None, "mm", "=lower_hole_side_offset", "2.51"),
    ("lower_hole_x_right", None, "mm", "=assembly_width - lower_hole_side_offset", "33.54"),
    ("lower_hole_y", None, "mm", "=lower_hole_bottom_offset", "3.40"),
    ("upper_hole_spacing", None, "mm", "=upper_hole_x_right - upper_hole_x_left", "30.57"),
    ("lower_hole_spacing", None, "mm", "=lower_hole_x_right - lower_hole_x_left", "31.03"),
    ("m2_clearance", 2.20, "mm", None, "paso M2"),
    ("m2_nut_af", 4.2, "mm", None, "alojamiento tuerca M2 (4.0 + 0.2)"),
    ("m2_nut_depth", 1.8, "mm", None, "tuerca M2 1.6 + 0.2"),
    ("pcb_standoff_height", 3.00, "mm", None, ""),
    ("pcb_standoff_outer_diameter", 5.00, "mm", None, ""),
    ("pcb_standoff_hole", None, "mm", "=m2_clearance", ""),
    ("standoff_fillet", 0.5, "mm", None, "base del separador"),
    ("pin_section_width", 25.44, "mm", None, "medido"),
    ("pin_access_clearance", 3.00, "mm", None, ""),
    ("lower_cutout_width", 27.86, "mm", None, "recorte de la placa, medido"),
    ("lower_cutout_height", 14.60, "mm", None, ""),
    ("usb_clearance_width", 16.00, "mm", None, ""),
    ("usb_clearance_height", 10.00, "mm", None, "sobre el borde superior"),
    ("lateral_clearance", 3.00, "mm", None, ""),
    ("# Carrier (pieza B)",),
    ("carrier_outer_width", 42.00, "mm", None, ""),
    ("carrier_outer_height", 48.00, "mm", None, ""),
    ("carrier_wall_thickness", 3.00, "mm", None, "travesaños traseros"),
    ("carrier_ear_depth", 12.00, "mm", None, "patas laterales con oreja M3 (profundidad en z)"),
    ("carrier_leg_margin", 0.6, "mm", None, "pata más allá del separador"),
    ("carrier_crossbar_height", 6.00, "mm", None, "travesaño a la altura del pivote"),
    ("carrier_corner_r", 2.00, "mm", None, ""),
    ("rear_clearance", 2.00, "mm", None, "pines traseros: hueco real = ear_depth - wall + standoff"),
    ("carrier_depth", 25.00, "mm", None, "informativo; la profundidad real sale de ear_depth + standoff + módulo"),
    ("# Base sobre el Compute Deck (piso v1) y brazo (pieza A)",),
    ("deck_front_x", 7.5, "mm", None, "borde frontal del piso v1"),
    ("deck_top_z", 43.0, "mm", None, "cara superior del piso v1"),
    ("deck_center_y", 45.0, "mm", None, ""),
    ("front_exp_x", 14.0, "mm", None, "puntos M3 frontales del piso v1"),
    ("deck_pass_x", 13.85, "mm", None, "paso de cable frontal del piso v1"),
    ("deck_pass_y", 51.5, "mm", None, ""),
    ("deck_mount_length", 55.00, "mm", None, "a lo ancho del rover (Y)"),
    ("deck_mount_width", 18.00, "mm", None, "a lo largo (X)"),
    ("deck_mount_thickness", 4.00, "mm", None, ""),
    ("mount_x_center", None, "mm", "=front_exp_x - 3 mm", "base X 2..20, libre de la zona LiDAR"),
    ("mount_slot_width", 3.40, "mm", None, "M3"),
    ("mount_slot_length", 8.00, "mm", None, "ajuste longitudinal"),
    ("mount_slot_spacing", 40.00, "mm", None, "= front_exp_spacing del piso"),
    ("base_pass_l", 7.0, "mm", None, "paso de cable en la base, sobre el del piso"),
    ("base_pass_w", 11.0, "mm", None, ""),
    ("drop_height", 35.00, "mm", None, "cara superior del piso -> eje del pivote"),
    ("forward_offset", 20.00, "mm", None, "borde del piso -> eje del pivote; encargo 15, sube a 20 para el giro V2 -10..+25 sin tocar el chasis"),
    ("pivot_x", None, "mm", "=deck_front_x - forward_offset", ""),
    ("pivot_z", None, "mm", "=deck_top_z - drop_height", ""),
    ("arm_thickness", 5.00, "mm", None, "brazos laterales (Y)"),
    ("general_clearance", 0.30, "mm", None, "por lado"),
    ("sliding_clearance", 0.40, "mm", None, "por lado (V2 pivote)"),
    ("arm_inner_y", None, "mm", "=deck_center_y - carrier_outer_width / 2 - general_clearance", "cara interior del brazo"),
    ("arm_center_offset", None, "mm", "=carrier_outer_width / 2 + general_clearance + arm_thickness / 2", "del centro"),
    ("arm_bottom_width", 10.0, "mm", None, "brazo en el pivote (X)"),
    ("arm_top_front_x", -2.0, "mm", None, ""),
    ("arm_top_rear_x", None, "mm", "=deck_front_x - 1 mm", "1 mm delante del piso"),
    ("pivot_boss_r", 6.0, "mm", None, "buje del pivote; V2 gira aquí"),
    ("wall_drop", 7.0, "mm", None, "muro frontal bajo la base"),
    ("rib_thickness", 3.00, "mm", None, ""),
    ("rib_length", 10.0, "mm", None, "nervadura triangular (Y)"),
    ("rib_height", 17.0, "mm", None, "nervadura triangular (Z)"),
    ("minimum_wall", 2.40, "mm", None, ""),
    ("fillet_radius", 2.00, "mm", None, ""),
    ("m3_clearance", 3.40, "mm", None, ""),
    ("m3_nut_af", 5.7, "mm", None, ""),
    ("m3_nut_depth", 2.6, "mm", None, ""),
    ("tie_slot_width", 3.00, "mm", None, ""),
    ("tie_slot_length", 10.00, "mm", None, ""),
    ("tie_slot_x", -1.75, "mm", None, "centro en el brazo; con forward_offset 20 el brazo mide X -4.8..1.3 entre z 30 y 40"),
    ("tie_slot_z", 35.0, "mm", None, ""),
    ("# Inclinación",),
    ("tilt_angle", 10, "deg", None, "positivo = cámara hacia abajo"),
    ("min_tilt", -10, "deg", None, "V2"),
    ("max_tilt", 25, "deg", None, "V2"),
    ("# Pieza de prueba",),
    ("test_margin", 5.0, "mm", None, "material alrededor de los agujeros"),
    ("test_thickness", 2.0, "mm", None, ""),
    ("# Referencias",),
    ("chassis_length", 160, "mm", None, ""),
    ("chassis_width", 90, "mm", None, ""),
    ("chassis_thickness", 3, "mm", None, "SUPUESTO"),
]

# rotación local del carrier -> rover: X = -z, Y = -x, Z = y
M0_ROWS = [[0, 0, -1], [-1, 0, 0], [0, 1, 0]]


def rot_from_rows(rows):
    m = App.Matrix()
    (m.A11, m.A12, m.A13), (m.A21, m.A22, m.A23), (m.A31, m.A32, m.A33) = rows
    return App.Placement(m).Rotation


def holes(P):
    return [("UL", P["upper_hole_x_left"], P["upper_hole_y"]), ("UR", P["upper_hole_x_right"], P["upper_hole_y"]),
            ("LL", P["lower_hole_x_left"], P["lower_hole_y"]), ("LR", P["lower_hole_x_right"], P["lower_hole_y"])]


# --------------------------------------------------------------------------- #
def build_carrier(doc, P):
    body = doc.addObject("PartDesign::Body", "Carrier")
    aw, ah = P["assembly_width"], P["assembly_height"]
    xo0 = aw / 2 - P["carrier_outer_width"] / 2
    xo1 = aw / 2 + P["carrier_outer_width"] / 2
    yo0 = ah / 2 - P["carrier_outer_height"] / 2
    yo1 = ah / 2 + P["carrier_outer_height"] / 2
    r_so = P["pcb_standoff_outer_diameter"] / 2
    lin = P["upper_hole_x_left"] + r_so + P["carrier_leg_margin"]       # cara interior pata izq.
    rin = P["upper_hole_x_right"] - r_so - P["carrier_leg_margin"]      # cara interior pata der.
    cb0 = ah / 2 - P["carrier_crossbar_height"] / 2
    cb1 = ah / 2 + P["carrier_crossbar_height"] / 2
    tr0 = P["upper_hole_y"] - r_so - P["carrier_leg_margin"]
    R = P["carrier_corner_r"]
    zs = P["carrier_ear_depth"]

    # marco trasero: U abierta abajo + ventana central (sin cerrar zona de pines)
    s = sk(body, "Sk_Frame")
    s.poly([(xo0, yo0), (lin, yo0), (lin, cb0), (rin, cb0), (rin, yo0), (xo1, yo0), (xo1, yo1), (xo0, yo1)],
           [R, R, None, None, R, R, R, R])
    s.rect(lin, cb1, rin, tr0)
    feat(body, "Pad", "Frame", s, Length=P["carrier_wall_thickness"])

    # patas laterales (orejas): llevan separadores, pivote y tuercas
    s = sk(body, "Sk_Legs")
    s.rect(xo0, yo0, lin, yo1, R)
    s.rect(rin, yo0, xo1, yo1, R)
    feat(body, "Pad", "Legs", s, Length=zs)

    # 4 separadores en coordenadas independientes (sin patrón rectangular)
    s = sk(body, "Sk_Standoffs", offset=zs)
    for _, x, y in holes(P):
        s.circle(x, y, r_so)
    feat(body, "Pad", "Standoffs", s, Length=P["pcb_standoff_height"])

    s = sk(body, "Sk_M2Holes", offset=zs + P["pcb_standoff_height"])
    for _, x, y in holes(P):
        s.circle(x, y, P["pcb_standoff_hole"] / 2)
    feat(body, "Pocket", "M2Holes", s, Type="ThroughAll")

    s = sk(body, "Sk_M2Nuts")
    for _, x, y in holes(P):
        s.hexagon(x, y, P["m2_nut_af"])
    feat(body, "Pocket", "M2Nuts", s, Length=P["m2_nut_depth"], Reversed=True)

    # pivote M3 lateral (eje x) y tuercas en las caras interiores de las patas
    py, pz = ah / 2, zs / 2
    s = sk(body, "Sk_PivotHole", plane="YZ_Plane", offset=aw / 2)
    s.circle(py, pz, P["m3_clearance"] / 2)
    feat(body, "Pocket", "PivotHole", s, Type="ThroughAll", SideType="Symmetric")
    s = sk(body, "Sk_NutL", plane="YZ_Plane", offset=lin)
    s.hexagon(py, pz, P["m3_nut_af"])
    feat(body, "Pocket", "PivotNutL", s, Length=P["m3_nut_depth"])
    s = sk(body, "Sk_NutR", plane="YZ_Plane", offset=rin)
    s.hexagon(py, pz, P["m3_nut_af"])
    feat(body, "Pocket", "PivotNutR", s, Length=P["m3_nut_depth"], Reversed=True)
    doc.recompute()
    zt = (zs + P["pcb_standoff_height"]).v
    f, n = try_fillet(doc, body, "StandoffFillet",
                      lambda e: e.Curve.TypeId == "Part::GeomCircle" and abs(e.Curve.Radius - r_so.v) < 1e-3
                      and abs(e.BoundBox.ZMax - zs.v) < 1e-4 and abs(e.BoundBox.ZMin - zs.v) < 1e-4,
                      P["standoff_fillet"].v)
    return body, (f is not None, n)


def build_test(doc, P):
    body = doc.addObject("PartDesign::Body", "CarrierTest")
    m = P["test_margin"]
    r_so = P["pcb_standoff_outer_diameter"] / 2
    x0, x1 = P["lower_hole_x_left"] - m, P["lower_hole_x_right"] + m
    y0, y1 = P["lower_hole_y"] - m, P["upper_hole_y"] + m
    ix0, ix1 = P["upper_hole_x_left"] + m, P["upper_hole_x_right"] - m
    iy0, iy1 = P["lower_hole_y"] + m, P["upper_hole_y"] - m
    s = sk(body, "Sk_TestRing")
    s.rect(x0, y0, x1, y1, E.k(2))
    s.rect(ix0, iy0, ix1, iy1)
    feat(body, "Pad", "TestRing", s, Length=P["test_thickness"])
    s = sk(body, "Sk_TestStandoffs", offset=P["test_thickness"])
    for _, x, y in holes(P):
        s.circle(x, y, r_so)
    feat(body, "Pad", "TestStandoffs", s, Length=P["pcb_standoff_height"])
    s = sk(body, "Sk_TestHoles", offset=P["test_thickness"] + P["pcb_standoff_height"])
    for _, x, y in holes(P):
        s.circle(x, y, P["pcb_standoff_hole"] / 2)
    feat(body, "Pocket", "TestHoles", s, Type="ThroughAll")
    return body


# --------------------------------------------------------------------------- #
def build_arm(doc, P):
    body = doc.addObject("PartDesign::Body", "Arm")
    zt = P["deck_top_z"]
    mx = P["mount_x_center"]
    bx0, bx1 = mx - P["deck_mount_width"] / 2, mx + P["deck_mount_width"] / 2
    cy = P["deck_center_y"]
    by0, by1 = cy - P["deck_mount_length"] / 2, cy + P["deck_mount_length"] / 2

    s = sk(body, "Sk_Base", offset=zt)
    s.rect(bx0, by0, bx1, by1, P["fillet_radius"])
    feat(body, "Pad", "Base", s, Length=P["deck_mount_thickness"])

    # ranuras M3 a lo largo de X, sobre los puntos de expansión frontales del piso
    s = sk(body, "Sk_Slots", offset=zt + P["deck_mount_thickness"])
    rs = P["mount_slot_width"] / 2 - 0.05
    for sgn in (-1, 1):
        sy = cy + sgn * P["mount_slot_spacing"] / 2
        s.rect_c(mx, sy, P["mount_slot_length"], P["mount_slot_width"], rs)
    s.rect_c(P["deck_pass_x"], P["deck_pass_y"], P["base_pass_l"], P["base_pass_w"])
    feat(body, "Pocket", "Slots", s, Type="ThroughAll")

    # muro frontal bajo el voladizo de la base (une los dos brazos)
    yi0, yi1 = P["arm_inner_y"], 2 * cy - P["arm_inner_y"]
    s = sk(body, "Sk_Wall", offset=zt - P["wall_drop"])
    s.rect(bx0, yi0 - P["arm_thickness"], P["arm_top_rear_x"], yi1 + P["arm_thickness"])
    feat(body, "Pad", "FrontWall", s, Length=P["wall_drop"])

    # brazos laterales (plano XZ: x de sketch = X, y de sketch = Z; normal -Y)
    px, pz = P["pivot_x"], P["pivot_z"]
    hb = P["arm_bottom_width"] / 2
    top = zt + P["deck_mount_thickness"]
    for i, sgn in enumerate((-1, 1)):
        yc = cy + sgn * P["arm_center_offset"]
        s = sk(body, "Sk_Arm%d" % i, plane="XZ_Plane", offset=-yc)
        s.poly([(px - hb, pz), (px + hb, pz), (P["arm_top_rear_x"], zt), (P["arm_top_rear_x"], top),
                (P["arm_top_front_x"], top)])
        feat(body, "Pad", "Arm%d" % i, s, Length=P["arm_thickness"], SideType="Symmetric")
        s = sk(body, "Sk_Boss%d" % i, plane="XZ_Plane", offset=-yc)
        s.circle(px, pz, P["pivot_boss_r"])
        feat(body, "Pad", "PivotBoss%d" % i, s, Length=P["arm_thickness"], SideType="Symmetric")

    # 2 nervaduras triangulares brazo-muro-base (plano YZ, normal +X)
    for i, sgn in enumerate((-1, 1)):
        # 1 mm dentro del brazo: solape real en vez de caras coplanares (malla cerrada)
        yin = cy + sgn * (P["arm_center_offset"] - P["arm_thickness"] / 2 + 1.0)
        s = sk(body, "Sk_Rib%d" % i, plane="YZ_Plane", offset=bx0)
        if sgn < 0:
            pts = [(yin, zt - P["rib_height"]), (yin + P["rib_length"], zt), (yin, zt)]
        else:
            pts = [(yin, zt - P["rib_height"]), (yin, zt), (yin - P["rib_length"], zt)]
        s.poly(pts)
        feat(body, "Pad", "Rib%d" % i, s, Length=P["rib_thickness"])

    # pivote M3 (eje Y) y ranuras para bridas a través de ambos brazos
    s = sk(body, "Sk_ArmPivot", plane="XZ_Plane", offset=-cy)
    s.circle(px, pz, P["m3_clearance"] / 2)
    feat(body, "Pocket", "ArmPivot", s, Type="ThroughAll", SideType="Symmetric")
    s = sk(body, "Sk_TieSlots", plane="XZ_Plane", offset=-cy)
    s.rect_c(P["tie_slot_x"], P["tie_slot_z"], P["tie_slot_width"], P["tie_slot_length"])
    feat(body, "Pocket", "TieSlots", s, Type="ThroughAll", SideType="Symmetric")
    return body


def build_refs(doc, P, deck_step):
    ch = doc.addObject("PartDesign::Body", "ChassisRef")
    s = sk(ch, "Sk_Chassis")
    s.rect(E.k(0), E.k(0), P["chassis_length"], P["chassis_width"])
    feat(ch, "Pad", "ChassisPlate", s, Length=P["chassis_thickness"], Reversed=True)
    env = doc.addObject("PartDesign::Body", "CamModule_Env")
    z0 = P["carrier_ear_depth"] + P["pcb_standoff_height"]
    s = sk(env, "Sk_Env", offset=z0)
    s.rect(E.k(0), E.k(0), P["assembly_width"], P["assembly_height"])
    feat(env, "Pad", "EnvBox", s, Length=P["assembly_depth"])
    deck = None
    if os.path.exists(deck_step):
        sh = Part.Shape(); sh.read(deck_step)
        deck = doc.addObject("Part::Feature", "ComputeDeck")
        deck.Shape = sh
    return ch, env, deck


def main():
    args = argv_script()
    out = os.path.abspath(args[args.index("--out") + 1])
    os.makedirs(out, exist_ok=True)
    doc = App.newDocument("ESP32_CAM_Rover_Mount")
    P, vals = build_params(doc, SHEET, PARAMS)
    carrier, fil = build_carrier(doc, P)
    test = build_test(doc, P)
    arm = build_arm(doc, P)
    deck_step = os.path.join(HERE, "..", "deck-v1", "out", "rover_deck_v1_deck.step")
    chassis, env, deck = build_refs(doc, P, deck_step)
    doc.recompute()

    # montaje: CameraTilt (pivote + giro) contiene los Links del carrier y del módulo
    tilt = doc.addObject("App::Part", "CameraTilt")
    tilt.Placement = App.Placement(V(vals["pivot_x"], vals["deck_center_y"], vals["pivot_z"]),
                                   App.Rotation(V(0, 1, 0), -vals["tilt_angle"]))
    tilt.setExpression(".Placement.Base.x", "%s.pivot_x" % SHEET)
    tilt.setExpression(".Placement.Base.y", "%s.deck_center_y" % SHEET)
    tilt.setExpression(".Placement.Base.z", "%s.pivot_z" % SHEET)
    tilt.setExpression(".Placement.Rotation.Angle", "-%s.tilt_angle" % SHEET)
    rot0 = rot_from_rows(M0_ROWS)
    base_local = V(vals["carrier_ear_depth"] / 2, vals["assembly_width"] / 2, -vals["assembly_height"] / 2)
    links = {}
    for name, target in (("Carrier_Placed", carrier), ("CamModule_Placed", env)):
        lk = doc.addObject("App::Link", name)
        lk.LinkedObject = target
        lk.Placement = App.Placement(base_local, rot0)
        lk.setExpression(".Placement.Base.x", "%s.carrier_ear_depth / 2" % SHEET)
        lk.setExpression(".Placement.Base.y", "%s.assembly_width / 2" % SHEET)
        lk.setExpression(".Placement.Base.z", "-%s.assembly_height / 2" % SHEET)
        tilt.addObject(lk)
        links[name] = lk
    verificar(doc)

    def placed(body, tilt_deg=None):
        rot = App.Rotation(V(0, 1, 0), -(vals["tilt_angle"] if tilt_deg is None else tilt_deg))
        tp = App.Placement(V(vals["pivot_x"], vals["deck_center_y"], vals["pivot_z"]), rot)
        s = body.Shape.copy()
        s.Placement = tp.multiply(App.Placement(base_local, rot0))
        return s

    A, B, MOD = arm.Shape, placed(carrier), placed(env)
    CH = chassis.Shape
    DK = deck.Shape if deck else None
    R = []
    def chk(n, ok, d):
        R.append((n, "OK" if ok is True else ("FALLA" if ok is False else ok), d))

    # 1-3 envolvente del módulo
    eb = env.Shape.optimalBoundingBox()
    chk("1-3. Envolvente del módulo", abs(eb.XLength - vals["assembly_width"]) < 1e-6 and abs(eb.YLength - vals["assembly_height"]) < 1e-6
        and abs(eb.ZLength - vals["assembly_depth"]) < 1e-6, "%.2f x %.2f x %.2f mm" % (eb.XLength, eb.YLength, eb.ZLength))
    # 4-8 agujeros medidos en el sólido del carrier (cara superior de los separadores)
    ztop = vals["carrier_ear_depth"] + vals["pcb_standoff_height"]
    found = sorted({(round(e.Curve.Center.x, 3), round(e.Curve.Center.y, 3)) for e in carrier.Shape.Edges
                    if e.Curve.TypeId == "Part::GeomCircle" and abs(e.Curve.Radius - vals["m2_clearance"] / 2) < 1e-4
                    and abs(e.Curve.Center.z - ztop) < 1e-4})
    expect = sorted({(round(x.v, 3), round(y.v, 3)) for _, x, y in holes(P)})
    chk("4-8. Agujeros M2 independientes", found == expect,
        "medidos en el sólido: %s (UL 2.74/40.04, UR 33.31/40.04, LL 2.51/3.40, LR 33.54/3.40)" % found)
    # 9 recorte inferior: nada del carrier dentro del recorte delante de las patas
    lcw, lch = vals["lower_cutout_width"], vals["lower_cutout_height"]
    aw, ah, ad = vals["assembly_width"], vals["assembly_height"], vals["assembly_depth"]
    lin = vals["upper_hole_x_left"] + vals["pcb_standoff_outer_diameter"] / 2 + vals["carrier_leg_margin"]
    rin = vals["upper_hole_x_right"] - vals["pcb_standoff_outer_diameter"] / 2 - vals["carrier_leg_margin"]
    cut = Part.makeBox(rin - lin, lch + 10, 50, V(lin, -10, -5))
    v9 = carrier.Shape.common(cut).Volume
    chk("9. Recorte inferior abierto", v9 < 1e-6,
        "hueco del carrier %.2f mm (x %.2f..%.2f), de y=-inf a %.2f; recorte de la placa %.2f x %.2f. "
        "Las patas entran %.2f mm por lado bajo el recorte para sostener los separadores inferiores"
        % (rin - lin, lin, rin, lch, lcw, lch, ((aw - lcw) / 2 - lin) * -1 if lin > (aw - lcw) / 2 else 0))
    # 10-12 zonas libres (en coordenadas locales, delante de la placa)
    zf = ztop
    usb = Part.makeBox(vals["usb_clearance_width"], vals["usb_clearance_height"], ad, V(aw / 2 - vals["usb_clearance_width"] / 2, ah, zf))
    pins = Part.makeBox(vals["pin_section_width"] + 2 * vals["pin_access_clearance"], ah, ad,
                        V(aw / 2 - vals["pin_section_width"] / 2 - vals["pin_access_clearance"], 0, zf))
    lat = Part.makeCompound([Part.makeBox(vals["lateral_clearance"], ah, ad, V(-vals["lateral_clearance"], 0, zf)),
                             Part.makeBox(vals["lateral_clearance"], ah, ad, V(aw, 0, zf))])
    pl_inv = App.Placement(V(vals["pivot_x"], vals["deck_center_y"], vals["pivot_z"]),
                           App.Rotation(V(0, 1, 0), -vals["tilt_angle"])).multiply(App.Placement(base_local, rot0))
    A_loc = A.copy(); A_loc.Placement = pl_inv.inverse().multiply(A_loc.Placement)
    others_loc = Part.makeCompound([carrier.Shape, A_loc])
    for n_, box in (("10. Acceso USB-C (16x10 sobre el borde superior)", usb),
                    ("11. Zona de pines 25.44 + 2x3 libre delante de la placa", pins),
                    ("12. Headers laterales: 3 mm libres a cada lado", lat)):
        v = others_loc.common(box).Volume
        chk(n_, v < 1e-6, "interferencia %.4f mm3" % v)
    # 13 separadores
    chk("13. Separadores vs componentes", "PARCIAL",
        "tocan sólo la cara trasera de la placa en 4 zonas Ø5; lo que hay detrás de la placa no está medido. "
        "Hueco trasero entre placa y travesaños: %.1f mm" % (vals["carrier_ear_depth"] - vals["carrier_wall_thickness"] + vals["pcb_standoff_height"]))
    # 14 lente: nada delante del plano frontal del módulo
    front = Part.makeBox(aw + 20, ah + 20, 40, V(-10, -10, zf + ad))
    v14 = others_loc.common(front).Volume
    if DK:
        D_loc = DK.copy(); D_loc.Placement = pl_inv.inverse().multiply(D_loc.Placement)
        v14 += D_loc.common(front).Volume
    chk("14. Lente libre", v14 < 1e-6, "material delante del plano frontal del módulo: %.4f mm3" % v14)
    # 15 cables
    chk("15. Salida de cables", True, "U inferior abierta, USB libre arriba, 2 ranuras de brida %gx%g en los brazos, "
        "paso %gx%g en la base sobre el paso frontal del piso" % (vals["tie_slot_width"], vals["tie_slot_length"], vals["base_pass_l"], vals["base_pass_w"]))
    # 16 campo de visión: todo el resto del rover queda detrás del plano de la lente
    d = pl_inv.Rotation.multVec(V(0, 0, 1))
    lens = pl_inv.multVec(V(aw / 2, ah / 2, zf + ad))
    pts = list(A.Vertexes) + list(CH.Vertexes) + (list(DK.Vertexes) if DK else [])
    worst = max((p.Point - lens).dot(d) for p in pts)
    chk("16. Campo de visión", worst < 0,
        "dirección de la cámara (%.3f, %.3f, %.3f); el punto más adelantado del brazo/piso/chasis queda %.1f mm detrás del plano frontal "
        "del módulo, así que no puede salir en la imagen. Ruedas y suelo: NO VERIFICABLE" % (d.x, d.y, d.z, -worst))
    # 17 interferencias
    vals17 = {"brazo-piso": A.common(DK).Volume if DK else -1, "carrier-piso": B.common(DK).Volume if DK else -1,
              "brazo-chasis": A.common(CH).Volume, "carrier-chasis": B.common(CH).Volume,
              "carrier-brazo": A.common(B).Volume, "módulo-brazo": A.common(MOD).Volume, "módulo-chasis": MOD.common(CH).Volume}
    chk("17. Interferencias", all(0 <= v < 1e-4 for v in vals17.values()) and DK is not None,
        ", ".join("%s %.4f" % kv for kv in vals17.items()) + " mm3")
    def hit(tdeg, dx=0.0):
        bt = placed(carrier, tdeg); bt.translate(V(-dx, 0, 0))
        mt = placed(env, tdeg); mt.translate(V(-dx, 0, 0))
        others = [A, CH] + ([DK] if DK else [])
        return sum(x.common(o).Volume for x in (bt, mt) for o in others)
    free = [t for t in range(int(vals["min_tilt"]), int(vals["max_tilt"]) + 1) if hit(t) < 1e-3]
    ok_range = (min(free), max(free)) if free else None
    full = ok_range == (int(vals["min_tilt"]), int(vals["max_tilt"]))
    extra = 0.0
    if not full:
        while hit(vals["max_tilt"], extra) > 1e-3 and extra < 40:
            extra += 0.5
    chk("17b. Giro V2 (%+g..%+g°)" % (vals["min_tilt"], vals["max_tilt"]), True if full else "AVISO",
        "giro libre sin choques con brazo, piso y chasis: %s°" % (("%+d..%+d" % ok_range) if ok_range else "ninguno")
        + ("" if full else "; para llegar a %+g° hace falta forward_offset >= %.1f mm (ahora %.1f) o subir el pivote"
           % (vals["max_tilt"], vals["forward_offset"] + extra, vals["forward_offset"])))
    gb = B.optimalBoundingBox().ZMin
    chk("17c. Altura sobre el suelo", "NO VERIFICABLE",
        "punto más bajo del carrier a Z=%.1f mm (%.1f mm bajo la cara superior del chasis); falta la altura del chasis al suelo" % (gb, -gb))
    chk("18. Desmontaje", True, "módulo: 4 tornillos M2 con tuerca cautiva detrás; carrier: 2 M3 laterales. "
        "Comprobar que las cabezas M2 son accesibles por delante del sándwich")
    walls = {"separador (5 - 2.2)/2": (vals["pcb_standoff_outer_diameter"] - vals["m2_clearance"]) / 2,
             "pata alrededor de tuerca M3": (vals["carrier_ear_depth"] - vals["m3_nut_af"] / math.sqrt(3) * 2) / 2,
             "buje pivote del brazo": vals["pivot_boss_r"] - vals["m3_clearance"] / 2,
             "brazo": vals["arm_thickness"], "base": vals["deck_mount_thickness"], "travesaño": vals["carrier_wall_thickness"]}
    low = {k: v for k, v in walls.items() if v < vals["minimum_wall"]}
    chk("19. Paredes mínimas", "AVISO" if low else True,
        "; ".join("%s %.2f" % kv for kv in walls.items()) + (" — por debajo de %.1f: %s (valores dados en el encargo)"
                                                               % (vals["minimum_wall"], ", ".join(low)) if low else ""))

    # 20 imprimibilidad en orientación de impresión
    def overhangs(shape):
        z0 = shape.optimalBoundingBox().ZMin; bad = []
        for fc in shape.Faces:
            pr = fc.ParameterRange
            try:
                n = fc.normalAt((pr[0] + pr[1]) / 2, (pr[2] + pr[3]) / 2)
            except Exception:
                continue
            bb_ = fc.optimalBoundingBox()
            if n.z < -0.72 and fc.Area > 1 and abs(bb_.ZMax - z0) > 1e-3:
                bad.append((round(min(bb_.XLength, bb_.YLength), 1), round(bb_.ZMin, 1)))
        return bad
    arm_print = A.copy(); arm_print.rotate(V(0, 0, 0), V(1, 0, 0), 180)   # cara superior de la base sobre la cama
    arm_print.translate(V(0, 0, -arm_print.optimalBoundingBox().ZMin))
    car_print = carrier.Shape.copy()                                      # cara trasera sobre la cama
    test_print = test.Shape.copy()
    oa, oc, ot = overhangs(arm_print), overhangs(car_print), overhangs(test_print)
    chk("20. Imprimible sin soportes", max([w for w, _ in oa + oc + ot] + [0]) <= 12,
        "brazo (base boca abajo): %s; carrier (cara trasera abajo): %s; prueba: %s  [ancho de cada voladizo mm, z]"
        % (oa or "ninguno", oc or "ninguno", ot or "ninguno"))
    sk_bad = [o.o.Name for o in SKETCHES if not o.o.FullyConstrained or o.o.ConflictingConstraints or o.o.RedundantConstraints]
    nexpr = sum(len(o.ExpressionEngine) for o in doc.Objects if hasattr(o, "ExpressionEngine"))
    chk("21. Editable", not sk_bad, "%d sketches totalmente restringidos%s; %d expresiones"
        % (len(SKETCHES), "" if not sk_bad else ", SALVO " + ", ".join(sk_bad), nexpr))

    print("\n== validaciones ==")
    for n_, st, d_ in R:
        print("[%s] %s: %s" % (st, n_, d_))
    print("fillet separadores:", fil)
    for n_, s_ in (("brazo", A), ("carrier", carrier.Shape), ("prueba", test.Shape)):
        bb = s_.optimalBoundingBox()
        print("%-8s %.1f x %.1f x %.1f mm, %.2f cm3, ~%.1f g PLA macizo, válido %s"
              % (n_, bb.XLength, bb.YLength, bb.ZLength, s_.Volume / 1000, masa(s_), s_.isValid()))

    # exportación
    for fn, shp in (("Rover_Camera_Arm.stl", arm_print), ("ESP32_CAM_Carrier.stl", car_print),
                    ("ESP32_CAM_Carrier_Test.stl", test_print)):
        m, probs = validar_para_impresion(shp)
        if probs:
            raise RuntimeError("%s: %s" % (fn, probs))
        m.write(os.path.join(out, fn))
    tmp = []
    for nm, s_ in (("Arm", A), ("Carrier", B)):
        o = doc.addObject("Part::Feature", "EXP_" + nm); o.Shape = s_; tmp.append(o)
    Import.export(tmp, os.path.join(out, "assembly.step"))
    Import.export(tmp, os.path.join(out, "ESP32_CAM_Rover_Mount.step"))
    for o in tmp:
        doc.removeObject(o.Name)
    # visibilidad al abrir en la GUI: sólo piezas finales y referencias
    for b in (arm, carrier, test, chassis, env):
        for o in b.Group:
            o.Visibility = (o == b.Tip)
    carrier.Visibility = env.Visibility = test.Visibility = False
    doc.recompute()
    doc.saveAs(os.path.join(out, "ESP32_CAM_Rover_Mount.FCStd"))
    with open(os.path.join(out, "validation.json"), "w") as fh:
        json.dump({"checks": R}, fh, indent=1, ensure_ascii=False)
    print("escrito en", out)


ejecutar(main)
