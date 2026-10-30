# Open frame ESP32-CAM v2 (cam-mount-v2)

Soporte abierto para el conjunto ESP32-CAM más placa de expansión (36.05 × 41.89 × 22.69 mm). **La placa queda suspendida sobre 4 columnas M2**: ninguna otra parte del frame entra en su huella a menos de `standoff_height` (2.0 mm desde V5; antes 6.5) de su cara trasera, ni en las zonas de exclusión. Sustituye a [cam-mount-v1](../cam-mount-v1/README.md), que encajaba la placa en un carrier y chocaba con los pines.

**Estado:** verificado en CAD, **sin imprimir**. Imprime primero `ESP32_CAM_OpenFrame_Test_V5_2mm.stl` (separadores de 2 mm). `ESP32_CAM_Open_Frame_Test_V2.stl` es la prueba anterior con 6.5 mm. Con 2 mm, las orejas del frame completo (11 mm de profundidad) sobresalen 3 mm por detrás del plano de la placa, junto a sus bordes laterales (validación 6b).

## Archivos (`out/`)
| Archivo | Contenido |
|---|---|
| `ESP32_CAM_Open_Frame.FCStd` | Modelo paramétrico (hoja `ROVER_ESP32_CAM_MOUNT`): `Arm`, `OpenFrame`, `FrameTest`, ensamblaje `CameraTilt` y referencias (`ComputeDeck`, `ChassisRef`, `CamModule_Env`, `Keepouts`) |
| `ESP32_CAM_Open_Frame.stl` / `.step` | Pieza B, el frame abierto; se imprime con la cara trasera sobre la cama |
| `Rover_Camera_Arm.stl` | Pieza A, la base y el brazo; se imprime con la base boca abajo |
| `ESP32_CAM_OpenFrame_Test_V5_2mm.stl` | El mismo frame sin orejas y con separadores de 2 mm, para probar el módulo (44 × 50 × 8 mm, unos 5.0 g) |
| `assembly.step` | Piezas A y B en coordenadas del rover, con 10° de inclinación |
| `ESP32_CAM_Open_Frame_plano.pdf` | Plano BL-CAM-002: frame frontal, trasero y lateral a 2:1, brazo a 1:1 y tabla del patrón M2 |
| `renders/` | Vistas isométrica, frontal, lateral, superior, trasera y explosionada, más las zonas de exclusión, el frame de frente y la pieza de prueba |

## Regenerar
```bash
F=~/.claude/skills/freecad/scripts/fc.sh
$F run build_open_frame.py -- --out "$PWD/out"
$F run render_views.py -- --dir "$PWD/out"
$F run make_drawing.py -- --model "$PWD/out/ESP32_CAM_Open_Frame.FCStd" --out "$PWD/out"
$F gui export_pdf.py -- --dir "$PWD/out"
$F gui fix_display.py -- --file "$PWD/out/ESP32_CAM_Open_Frame.FCStd"   # visibilidad al abrir
```

## Patrón M2
- **A y B** (datum, ya validados): (2.74, 40.04) y (33.31, 40.04), agujeros de Ø2.2.
- **C y D** se calculan **en la hoja** con A-C, B-D, C-D y A-D (fórmulas con `sqrt`), no a partir de los bordes de la placa: C = (6.40, 5.70) y D = (28.61, 5.83).
- Las 5 medidas no son del todo coherentes: si el patrón fuera simétrico, C y D quedarían unos 0.5 mm más a la derecha, pero entonces A-D daría 43.26 en vez de 42.89. Las ranuras horizontales de 5 × 2.3 mm dan ±1.35 mm de ajuste y cubren las dos soluciones.
- Cada M2 lleva tuerca cautiva detrás: hexágono en A y B, y en C y D un canal de 7.6 mm para que la tuerca acompañe la ranura. Tornillos M2 de unos 16 mm.

## Zonas de exclusión (objeto `Keepouts`)
Filas de pines + 3 mm, pines superiores, header inferior, USB-C (16 × 12 mm sobre el borde superior) y 3 mm a cada lado. El frame no entra en ninguna; las columnas son la única excepción, y solo en sus agujeros. **Las posiciones de los pines son supuestas** y hay que medirlas:
- `pin_row_center_x`: filas centradas en la placa;
- `pin_row_y0` y `pin_row_y1`: de 10 a 34 mm en vertical;
- `pin_tail_length`: 3 mm hacia atrás;
- `top_pin_zone_height` y `bottom_pin_zone_height`: 8 y 12 mm.

Si cambias estos valores, `build_open_frame.py` vuelve a comprobar todas las zonas.

## Estructura
- **Montantes laterales:** 4 × 6 mm, completamente fuera de la huella de la placa.
- **Esquinas:** solo llegan hasta los agujeros.
- **Puente inferior:** 3 mm, por debajo del borde de la placa y fuera de su huella. Es la única pieza transversal, y queda a 9.5 mm detrás de la placa.
- **Orejas laterales:** 2 M3 por lado (pivote y bloqueo, separados 9 mm), así la unión V1 es **rígida**. Para hacerla ajustable en una V2 basta con cambiar el agujero de bloqueo por una ranura en arco. Con este brazo, el frame giraría sin choques entre −10° y +16°.
- **Base:** 55 × 24 × 4 mm con 2 ranuras M3 de 3.4 × 10 mm separadas 40 mm, sobre los puntos de expansión frontales del piso v1. Las ranuras están descentradas para alcanzar X=14 sin pisar los M3 del LiDAR.
- **Brazo:** 2 nervaduras triangulares, muro frontal y 2 ranuras para bridas de 3 × 10 mm.

## Cambios respecto al encargo
| Parámetro | Encargo | Modelo | Motivo |
|---|---|---|---|
| `arm_offset` | 15 | **16.5** | Con 15, el borde inferior del frame toca el chasis con la inclinación de 10° (0.02 mm³). Con 16.5 queda a 1.45 mm |
| fillet en la base de las columnas | sí | no | Las columnas nacen en la cama; el fillet de PartDesign dejaba el sólido inválido |
| paredes de las columnas | 2.4 mín. | 1.65 | Sale de Ø5.5 con agujero de 2.2, los valores del encargo. Sube `standoff_outer_diameter` si quieres más margen |

## Pendientes
Medir dónde están realmente los pines (ver arriba), la altura del chasis al suelo (el frame baja hasta 16.7 mm por debajo de la cara superior del chasis) y comprobar el acceso a las cabezas M2. La rigidez no está simulada.
