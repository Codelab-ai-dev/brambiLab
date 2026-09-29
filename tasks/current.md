# Punto de reanudación
Actualizado: 2026-09-28

Foundation versionada y licenciada en https://github.com/Codelab-ai-dev/brambiLab (repositorio público). FND-001 y FND-002 terminadas.

Prioridad actual: WEB-008 en curso: estado en [Issue #37](https://github.com/Codelab-ai-dev/brambiLab/issues/37), rama `chore/WEB-008-operations`. Primero lo reproducible (configuración/release, backup/restauración, validación); las acciones en el VPS, DNS, destino de backup y Resend real esperan autorización y datos de Gustavo. WEB-007 terminada: [Issue #34](https://github.com/Codelab-ai-dev/brambiLab/issues/34) cerrado, PR #35 (recepción, cola y adaptador) y #36 (formulario, panel y e2e) unidos en `main`; **contacto real pendiente** (desactivado, [guía](../docs/operations/contacto-resend.md)). WEB-006 terminada: [Issue #30](https://github.com/Codelab-ai-dev/brambiLab/issues/30) cerrado, PR #31 (API pública y búsqueda), #32 (SSR, i18n y SEO) y #33 (configuración, integración y e2e) unidos en `main`. WEB-005 terminada: [Issue #26](https://github.com/Codelab-ai-dev/brambiLab/issues/26) cerrado, PR #27 (modelo/API/rutas), #28 (scheduler) y #29 (panel y e2e) unidos en `main`. WEB-004 terminada: [Issue #20](https://github.com/Codelab-ai-dev/brambiLab/issues/20) cerrado, PR #21 (almacenamiento/API), #22 (entrega autorizada) y #23 (panel, editor y e2e) unidos en `main`. Portada pública rediseñada («banco de pruebas vivo», PR #24). WEB-003 terminada: [Issue #12](https://github.com/Codelab-ai-dev/brambiLab/issues/12), PR #15 (modelo/API), #16 (editor/Markdown) y #17 (panel/e2e), integrados en `main` con el PR de integración desde `feat/WEB-003-editor` (#16 y #17 se habían unido en ramas intermedias). WEB-001 y WEB-002 terminadas (#7, #11, #13). Especificación: [web-v1.md](../docs/architecture/web-v1.md); arranque y verificación en [platform/README.md](../platform/README.md). Web priorizada por Gustavo sin esperar a completar BL-001. FND-003 sigue en review.

En curso independiente: BL-001-001 (inventario), preparación ya unida en `main`. La plantilla del inventario está lista; la tarea espera los datos físicos de Gustavo.
Consulta [backlog.md](backlog.md) para aceptación y estado. Antes de modificar, lee [AGENTS.md](../AGENTS.md).

## Información necesaria para BL-001-001
No inferir estos datos de la planificación anterior. Fotos según [conventions.md](../docs/conventions.md) (nombre con fecha, sin EXIF).
1. Control: placa actual, fotos de ambas caras con la serigrafía legible. ¿Hay algún ESP32 disponible y de qué variante?
2. Potencia: modelo del driver y foto de su cableado; modelo o etiqueta de los motores y cuántos son.
3. Batería: química, tensión nominal, capacidad, conector y cargador. ¿Hay interruptor, fusible o BMS?
4. Funcionamiento actual: con qué se controla hoy (mando, app, nada) y qué funciona. Un vídeo corto con las ruedas levantadas sirve como prueba V0.
5. Mecánica: medidas del chasis (largo, ancho y alto), separación entre fijaciones, espacio libre para el teléfono y tipo de ruedas o transmisión.
6. Teléfono y soporte: disponibilidad del CMF Phone 1, ubicación de los CAD/STL del soporte pan/tilt y modelos de servos si los hay.

## Última entrega
2026-09-27 · BL-001-001 (preparación): hardware.md y bom.md alineados y ampliados con los elementos que faltaban; convención de nombres y limpieza EXIF para fotos; `.gitignore` para multimedia y builds; prueba de corte físico propuesta en validation.md; enlaces de los índices de docs/. Verificado: enlaces Markdown relativos. Sin pruebas físicas.

Anteriores: FND-001 (commit inicial `332e5f0`) y FND-002 (licencias, [ADR-007](../docs/architecture/ADR-007-licencias.md)).

## Entregas web
2026-09-28 · #41 imágenes del contenido en tarjetas compactas con visor accesible y corrección del 500 del documento vacío. [Handoff](../docs/handoffs/2026-09-28-ui-41-image-viewer.md).

2026-09-28 · WEB-008 entrega 1 (configuración de producción, mantenimiento, preflight y runbook de release). [Handoff](../docs/handoffs/2026-09-28-web-008-config.md); gates en [lanzamiento.md](../docs/operations/lanzamiento.md). Unida (#38).
2026-09-28 · WEB-008 entrega 2 (backup cifrado externo y restauración aislada, probados contra un destino simulado). [Handoff](../docs/handoffs/2026-09-28-web-008-backup.md); runbook [backup.md](../docs/operations/backup.md). Unida (#39).
2026-09-28 · WEB-008 entrega 3 (chequeos, avisos por webhook desactivados, página «Operación», incidentes, medición de carga y lista de lanzamiento). [Handoff](../docs/handoffs/2026-09-28-web-008-alerts.md). Parte reproducible completa; el resto espera acceso y decisiones de Gustavo ([gates](../docs/operations/lanzamiento.md)).

2026-09-28 · WEB-007 PR 1 (recepción, antispam, cola y adaptador Resend; contacto desactivado hasta configurarlo). [Handoff](../docs/handoffs/2026-09-28-web-007-api.md); guía [contacto-resend.md](../docs/operations/contacto-resend.md). PR 1 #35 unido; PR 2 #36 (formulario, panel y e2e; [handoff](../docs/handoffs/2026-09-28-web-007-form.md)) unido. WEB-007 terminada; Resend real pendiente (gate de WEB-008).

2026-09-28 · WEB-006: PR 1 #31 (API pública, búsqueda y configuración; [handoff](../docs/handoffs/2026-09-28-web-006-api.md)) unido; PR 2 #32 (sitio SSR, i18n y SEO; [handoff](../docs/handoffs/2026-09-28-web-006-ssr.md)) unido; PR 3 #33 (configuración del sitio, integración y e2e; [handoff](../docs/handoffs/2026-09-28-web-006-integration.md)) unido. WEB-006 terminada.

2026-09-28 · WEB-005: PR 1 #27 (modelo, API, rutas y lector público; [handoff](../docs/handoffs/2026-09-28-web-005-api.md)) unido; PR 2 #28 (ejecución programada; [handoff](../docs/handoffs/2026-09-28-web-005-scheduler.md)) unido; PR 3 #29 (panel y e2e; [handoff](../docs/handoffs/2026-09-28-web-005-panel.md)) unido. WEB-005 terminada.

2026-09-28 · WEB-004 en tres PR: #21, #22 y el panel. Handoffs: [almacenamiento](../docs/handoffs/2026-09-28-web-004-storage.md), [entrega](../docs/handoffs/2026-09-28-web-004-delivery.md), [panel](../docs/handoffs/2026-09-28-web-004-panel.md).

2026-09-27 · WEB-003 en tres PR apilados: #15 (modelo y API), #16 (editor y Markdown) y el panel con e2e. Handoffs: [API](../docs/handoffs/2026-09-27-web-003-api.md), [editor](../docs/handoffs/2026-09-27-web-003-editor.md), [panel](../docs/handoffs/2026-09-27-web-003-panel.md).

2026-09-27 · WEB-002: login del propietario con GitHub, sesiones, CSRF, proxy de entrada y e2e con GitHub simulado. [Handoff](../docs/handoffs/2026-09-27-web-002.md).

2026-09-27 · WEB-001 seguimiento: credenciales PostgreSQL mediante variables `PG*` y actualización del seguimiento tras la revisión de Codex. [Handoff](../docs/handoffs/2026-09-27-web-001-followup.md).

2026-09-27 · WEB-001: scaffold SSR/Go/PostgreSQL/Compose, OpenAPI base, migraciones y CI. [Handoff](../docs/handoffs/2026-09-27-web-001.md).

2026-09-27: requisitos de la entrevista plasmados en especificación y ADR-006; roadmap y alcance alineados. [Handoff](../docs/handoffs/2026-09-27-web-v1.md). Destino externo de backup pendiente antes de lanzar.
