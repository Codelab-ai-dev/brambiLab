# Backlog inicial
Fuente de estado hasta migrar a GitHub Issues. Prioridad por orden; no son Issues remotos creados.

## FND-001 — Revisar Foundation y primer commit
Estado: done (2026-09-27, commit `332e5f0`; remoto público Codelab-ai-dev/brambiLab). Responsable: Gustavo con agente local.
Aceptación: revisar alcance y convenciones; crear commit local; registrar hash y elegir posteriormente remoto/visibilidad. Evidencia: hash real. No crear historial ficticio.

## BL-001-001 — Inventario y estado inicial V0
Estado: in-progress (plantilla lista; espera datos físicos de Gustavo, ver [current.md](current.md)). Dependencia: ninguna técnica.
Alcance: completar hardware.md, bom.md y build-log.md con datos observados.
Aceptación: identificar placa, driver, motores y alimentación; documentar control actual; fotos referenciadas; dimensiones de montaje; cada desconocido explícito; descripción de una prueba del estado inicial o bloqueo que impide ejecutarla.
Fuera: comprar componentes, diseñar electrónica nueva, implementar firmware.

## BL-001-002 — Arquitectura del primer control y parada
Estado: backlog. Depende: BL-001-001.
Aceptación: diagrama de conexiones con pinout verificado; límites de potencia sustentados en fichas técnicas; comandos y estados; timeout y parada definidos; plan de prueba con ruedas levantadas. Registrar ADR cuando corresponda.

## BL-001-003 — Control ESP32 mínimo
Estado: backlog. Depende: BL-001-002.
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
| UI-45 | Logo modular en cabecera, pie y favicon | En revisión ([#45](https://github.com/Codelab-ai-dev/brambiLab/issues/45)); variante oscura pendiente de aprobación | Kit de marca de Gustavo |

Seguimiento de la revisión de Codex a WEB-001 ([comentario](https://github.com/Codelab-ai-dev/brambiLab/pull/5#issuecomment-5861621351)):
- Credenciales PostgreSQL con caracteres reservados: corregido en `fix/WEB-001-db-credentials` con variables `PG*` y prueba de integración.
- Aceptación adicional de WEB-002: definir y verificar que `/api/v1/*` y `/media/*` llegan a Go bajo el mismo origen público con el enrutado de producción elegido, no sólo con el proxy de Vite. El SSR reenvía la cookie de sesión únicamente a la API interna de confianza; Go conserva autenticación y autorización. Login, callback y logout funcionan; el propietario es aceptado y cualquier otra cuenta rechazada; preview y admin exigen sesión; cookies, CSRF y respuestas privadas `no-store` probados.

BL-001-001 conserva su progreso; no requiere terminarse antes de implementar la web.
