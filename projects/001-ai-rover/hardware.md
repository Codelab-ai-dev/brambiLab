# Inventario físico
No completar modelos por parecido visual. Los elementos usan los mismos nombres que [bom.md](bom.md).

| Elemento | Estado documental | Dato requerido |
|---|---|---|
| Chasis | Declarado por Gustavo | Dimensiones, fijaciones, fotos actuales |
| Ruedas y transmisión | Pendiente | Tipo (ruedas u orugas), reductora, encoders si existen |
| Motores | Pendiente | Modelo, tensión y cantidad confirmada |
| Driver de motores | L298N, declarado por Gustavo | Confirmar modulo concreto y estado de los jumpers ENA/ENB |
| Placa de control actual | ESP32-CAM, verificada por lectura del chip 2026-10-09 | Fotos de ambos lados |
| ESP32 | ESP32-CAM AI-Thinker (ESP32-D0WD-V3 rev3.1, 4 MB flash, OV2640), MAC 3c:8a:1f:c2:40:d4 | Confirmar que la serigrafia dice AI-Thinker |
| Antena WiFi | Externa por conector IPEX (2026-10-23: suelta daba −80 a −88 dBm; conectada, −38 a −44 dBm) | Foto de la antena y de la resistencia de selección |
| Control remoto actual | Pendiente | Mando, receptor o app con que se maneja hoy; o indicar que no existe |
| Batería | Pendiente | Química, tensión nominal, capacidad y conector |
| Cargador | Pendiente | Modelo y compatibilidad con la química de la batería |
| Interruptor y protección | Pendiente | Si hay interruptor general, fusible o BMS, y dónde |
| Cableado y conectores | Pendiente | Fotos del recorrido y tipo de conectores entre batería, driver y motores |
| CMF Phone 1 | Integración prevista | Disponibilidad, orientación y sujeción |
| Soporte impreso | Diseño previo no inspeccionado | Fuente CAD, STL y prueba de ajuste |
| Servos | Pendiente | Modelos, cantidad, alimentación y consumo |

## Captura inicial
Fotografiar conjunto superior/lateral y etiquetas. Medir separación entre fijaciones y espacio libre. Registrar cómo se controla actualmente sin modificar conexiones. Identificar alimentación antes de proponer nuevas conexiones. Nombrar y limpiar las fotos según [conventions.md](../../docs/conventions.md) y guardarlas en [media/](media/README.md).
