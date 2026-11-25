# Soporte frontal ESP32-CAM v1 (cam-mount-v1)

Soporte abierto y desmontable para el conjunto ESP32-CAM más placa de expansión (36.05 × 41.89 × 22.69 mm, medido con calibre). Se monta en la parte frontal del piso superior v1 ([deck-v1](../deck-v1/README.md)).

**Estado:** diseño verificado en CAD, **sin imprimir**. Antes de imprimir el soporte completo, imprime `ESP32_CAM_Carrier_Test.stl` y comprueba que los 4 M2 entran.

## Archivos (`out/`)
| Archivo | Contenido |
|---|---|
| `ESP32_CAM_Rover_Mount.FCStd` | Modelo paramétrico. Hoja `ROVER_CAM_PARAMETERS`, cuerpos `Arm`, `Carrier` y `CarrierTest`, y ensamblaje `CameraTilt` (pivote más inclinación) con referencias al piso, el chasis y la envolvente del módulo |
| `Rover_Camera_Arm.stl` | Pieza A: base, muro, brazos y nervaduras. Orientada con la cara superior de la base sobre la cama |
| `ESP32_CAM_Carrier.stl` | Pieza B: carrier. Orientada con la cara trasera sobre la cama |
| `ESP32_CAM_Carrier_Test.stl` | Anillo de 2 mm con los 4 separadores, para probar el patrón M2 (unos 3.6 g) |
| `ESP32_CAM_Rover_Mount.step`, `assembly.step` | Piezas A y B colocadas en coordenadas del rover, con la inclinación V1 de 10° |
| `ESP32_CAM_Rover_Mount_plano.pdf` | Plano BL-CAM-001 (A3): carrier a 2:1 con el patrón M2 acotado y brazo a 1:1 |
| `renders/` | Vistas isométrica, frontal, lateral, superior, trasera, explosionada, de la pieza de prueba y del carrier |
| `validation.json` | Resultado de las validaciones |

## Regenerar
```bash
F=~/.claude/skills/freecad/scripts/fc.sh
$F run build_cam_mount.py -- --out "$PWD/out"
$F run render_views.py -- --dir "$PWD/out"
$F run make_drawing.py -- --model "$PWD/out/ESP32_CAM_Rover_Mount.FCStd" --out "$PWD/out"
$F gui export_pdf.py -- --dir "$PWD/out"
```
`build_cam_mount.py` usa `../common/fcparam.py` y lee el piso en `../deck-v1/out/rover_deck_v1_deck.step`. Probé a editar la hoja (`tilt_angle` a 20°, `lower_hole_side_offset` a 2.8 y `drop_height` a 30): todo se recalcula sin errores y los sketches siguen restringidos.

## Diseño
- **Montaje en el piso:** base de 55 × 18 × 4 mm con 2 ranuras M3 de 3.4 × 8 mm separadas 40 mm, que caen sobre los **puntos M3 de expansión frontal del piso v1** (X=14, Y=25 y 65). La base ocupa X 2..20, delante de la zona LiDAR, y lleva un paso de 7 × 11 mm encima del paso de cable frontal del piso.
- **Brazo (pieza A):** dos brazos laterales de 5 mm unidos por un muro frontal y 2 nervaduras triangulares. Acaban en un buje de Ø12 con un M3 pasante, que es el pivote. Cada brazo tiene una ranura para brida de 3 × 10 mm.
- **Carrier (pieza B):** marco trasero en forma de H, abierto por abajo (forma de U) y con una ventana central, así que no tapa los pines. Las dos patas laterales miden 12 mm de profundidad y llevan los **4 separadores M2 en coordenadas independientes** (2.74/40.04, 33.31/40.04, 2.51/3.40 y 33.54/3.40), con tuercas M2 cautivas por detrás. El pivote es un M3 lateral con la tuerca en la cara interior de cada pata.
- **Desmontaje:** el módulo sale quitando 4 M2; el carrier, quitando 2 M3.
- **Inclinación:** 10° hacia abajo (`tilt_angle`; positivo significa hacia abajo). Se aplica girando todo el carrier sobre el eje del pivote. La V2 ajustable solo necesita aflojar los M3: el rango de −10° a +25° está comprobado sin choques.

## Cambios respecto al encargo (todos son parámetros)
| Parámetro | Encargo | Modelo | Motivo |
|---|---|---|---|
| `forward_offset` | 15 | **20** | Con 15, el carrier choca con el borde del chasis a +10°, y solo gira libre entre −8° y +7°. Con 20 cabe todo el rango de −10° a +25° |
| profundidad de las patas | — | 12 | Con 10 mm, la pared junto a la tuerca M3 quedaba en 1.7 mm; con 12 queda en 2.7 mm |
| `carrier_depth` | 25 | informativo | La profundidad real sale de las patas (12) más el separador (3) más el módulo (22.69) |

## Supuestos y pendientes
1. **Frente del rover en X=0**, igual que en el piso v1.
2. **Altura al suelo: no verificada.** El carrier baja hasta 16.7 mm por debajo de la cara superior del chasis. Mide la distancia de esa cara al suelo; si es poca, reduce `drop_height`.
3. **Lo que hay detrás de la placa de expansión** (puntas de los pines, soldaduras) no está medido. Entre la placa y los travesaños quedan 12 mm, y los separadores solo tocan 4 zonas de Ø5.
4. **Cabezas M2:** comprueba que se alcanzan por delante del sándwich con la ESP32-CAM puesta.
5. **Pared de los separadores:** (5 − 2.2) / 2 = 1.4 mm, por debajo del mínimo de 2.4 mm. Los valores son del encargo y el separador no es estructural, pero si quieres más margen sube `pcb_standoff_outer_diameter` a 6.
6. **Lente:** no conozco su posición exacta. Delante del módulo no hay material, y todo el brazo, el piso y el chasis quedan unos 30 mm por detrás de su cara frontal, así que no pueden salir en la imagen. Las ruedas no están modeladas.
