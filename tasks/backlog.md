# Backlog inicial
Fuente de estado hasta migrar a GitHub Issues. Prioridad por orden; no son Issues remotos creados.

## FND-001 — Revisar Foundation y primer commit
Estado: done (2026-09-27, commit `332e5f0`; remoto público Codelab-ai-dev/brambiLab). Responsable: Gustavo con agente local.
Aceptación: revisar alcance y convenciones; crear commit local; registrar hash y elegir posteriormente remoto/visibilidad. Evidencia: hash real. No crear historial ficticio.

## BL-001-001 — Inventario y estado inicial V0
Estado: in-progress (2026-10-13). Hecho: placa de control identificada por lectura del chip (ESP32-CAM, ESP32-D0WD-V3) con volcado del flash de fábrica; control previo observado (`CameraWebServer` de ejemplo, sin control de motores); driver L298N declarado por Gustavo. Faltan fotos, motores, batería, cargador, dimensiones y la prueba V0 con motores (ver [current.md](current.md)). Dependencia: ninguna técnica.
Alcance: completar hardware.md, bom.md y build-log.md con datos observados.
Aceptación: identificar placa, driver, motores y alimentación; documentar control actual; fotos referenciadas; dimensiones de montaje; cada desconocido explícito; descripción de una prueba del estado inicial o bloqueo que impide ejecutarla.
Fuera: comprar componentes, diseñar electrónica nueva, implementar firmware.

## BL-001-002 — Arquitectura del primer control y parada
Estado: in-progress (2026-10-13, firmware unido en `main` con [PR #56](https://github.com/Codelab-ai-dev/brambiLab/pull/56)). Depende: BL-001-001, que sigue abierta; Gustavo adelantó el trabajo con el hardware ya identificado. Hecho: comandos y estados, timeout de 500 ms y parada al caer el WiFi, plan de prueba con ruedas levantadas ([validation.md](../projects/001-ai-rover/validation.md)). Falta: pinout verificado (hoy sale del instructivo del kit), límites de potencia con fichas técnicas, diagrama de conexiones y ADR si corresponde.
Aceptación: diagrama de conexiones con pinout verificado; límites de potencia sustentados en fichas técnicas; comandos y estados; timeout y parada definidos; plan de prueba con ruedas levantadas. Registrar ADR cuando corresponda.

## BL-001-003 — Control ESP32 mínimo
Estado: review (2026-10-13; firmware unido en `main` con [PR #56](https://github.com/Codelab-ai-dev/brambiLab/pull/56)). Depende: BL-001-002. Hecho: firmware `rover_esp32cam` compilable con `arduino-cli`; ronda 1 sin potencia en motores (rechazo de comandos inválidos y parada por pérdida de enlace en `/status`); ronda 2 el 2026-10-09 en el suelo con batería, según informe verbal de Gustavo: avance, retroceso, giro y parada correctos, sin brownout ([validation.md](../projects/001-ai-rover/validation.md)). Falta para cerrar: que Gustavo acepte el informe verbal como evidencia o grabe vídeo; arranque parado y pérdida de enlace observados con motores alimentados.
Aceptación: firmware compilable con placa/toolchain registrados; avance, retroceso, giro y parada; arranque parado; rechazo de comandos inválidos; pérdida de enlace lleva a parada dentro del timeout acordado; evidencia de prueba física por Gustavo. Elegir transporte de prueba en la especificación previa.

## BL-001-004 — Documentar primer hito reproducible
Estado: backlog. Depende: BL-001-003.
Aceptación: instrucciones desde cero, versión, cableado, BOM, vídeo o fotos, pruebas, fallos y siguiente paso; diferenciar demostrado de planeado.

## RES-001 — Evaluar enlace Android–ESP32
Estado: backlog. Depende: BL-001-001.
Aceptación: comparar Wi-Fi, BLE y USB cuando sean compatibles con los modelos reales; latencia requerida, recuperación y esfuerzo; fuentes fechadas; proponer ADR-005 sin darlo por aprobado.

## FND-002 — Definir licencias y publicación
Estado: done (2026-09-27, [ADR-007](../docs/architecture/ADR-007-licencias.md)).
Aceptación: Gustavo elige términos para código, documentación y diseños; revisar derechos de recursos de terceros; registrar la decisión antes de etiquetar contenido como open source.

## FND-003 — Especificar web v1
Estado: review (2026-09-27). Sin dependencia del rover; prioridad cambiada por Gustavo.
Entregables: [especificación](../docs/architecture/web-v1.md) y [ADR-006](../docs/architecture/ADR-006-web-stack.md).
Aceptación: requisitos acordados, modelo de datos, API, publicación, despliegue y validación documentados. Implementación no iniciada.

## Implementación web prioritaria
Criterios completos en la sección 16 de la especificación. Los IDs siguientes son tareas documentales, no Issues remotos creados.

| ID | Tarea | Estado | Dependencia |
|---|---|---|---|
| WEB-001 | Scaffold SSR/API/DB/Docker y contrato base | done (PR #5, merge `c8abe1c`, CI run 36364615971) | Especificación |
| WEB-002 | GitHub OAuth de propietario y sesiones | Estado en [Issue #7](https://github.com/Codelab-ai-dev/brambiLab/issues/7) | WEB-001 |
| WEB-003 | Contenido, revisiones, i18n y editor | Estado en [Issue #12](https://github.com/Codelab-ai-dev/brambiLab/issues/12) | WEB-002 |
| WEB-004 | Archivos, descargas y videos preparados | Estado en [Issue #20](https://github.com/Codelab-ai-dev/brambiLab/issues/20) | WEB-003 |
| WEB-005 | Publicación manual/programada y retirada | Estado en [Issue #26](https://github.com/Codelab-ai-dev/brambiLab/issues/26) | WEB-004 |
| WEB-006 | Portafolio/laboratorio, SSR, SEO y búsqueda | Estado en [Issue #30](https://github.com/Codelab-ai-dev/brambiLab/issues/30) | WEB-005 |
| WEB-007 | Contacto con Resend | Estado en [Issue #34](https://github.com/Codelab-ai-dev/brambiLab/issues/34) | WEB-006 |
| WEB-008 | Coolify, backups y validación de lanzamiento | Cerrada con excepciones ([#37](https://github.com/Codelab-ai-dev/brambiLab/issues/37)); pendientes en [#43](https://github.com/Codelab-ai-dev/brambiLab/issues/43) | WEB-007 |
| UI-45 | Logo modular en cabecera, pie y favicon | Terminada ([#45](https://github.com/Codelab-ai-dev/brambiLab/issues/45) cerrado, PR #46) | Kit de marca de Gustavo |
| WEB-009 | Rediseño editorial del sitio público (navy, cyan, tipografía grande) | Terminada ([#47](https://github.com/Codelab-ai-dev/brambiLab/issues/47) cerrado; #48, #50, #51 en `main`); despliegue pendiente de autorización | UI-45 |
| UI-52 | Animaciones sutiles del diseño editorial | Terminada ([#52](https://github.com/Codelab-ai-dev/brambiLab/issues/52), PR #54) | WEB-009 |
| UI-53 | Hero 3D (nube de percepción con Three.js) | En PR ([#53](https://github.com/Codelab-ai-dev/brambiLab/issues/53)); sustituye el pulso de UI-52 | UI-52 |

Seguimiento de la revisión de Codex a WEB-001 ([comentario](https://github.com/Codelab-ai-dev/brambiLab/pull/5#issuecomment-5861621351)):
- Credenciales PostgreSQL con caracteres reservados: corregido en `fix/WEB-001-db-credentials` con variables `PG*` y prueba de integración.
- Aceptación adicional de WEB-002: definir y verificar que `/api/v1/*` y `/media/*` llegan a Go bajo el mismo origen público con el enrutado de producción elegido, no sólo con el proxy de Vite. El SSR reenvía la cookie de sesión únicamente a la API interna de confianza; Go conserva autenticación y autorización. Login, callback y logout funcionan; el propietario es aceptado y cualquier otra cuenta rechazada; preview y admin exigen sesión; cookies, CSRF y respuestas privadas `no-store` probados.

BL-001-001 conserva su progreso; no requiere terminarse antes de implementar la web.
