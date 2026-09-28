# Entrega de sesión
Fecha y zona: 2026-09-28, America/Mexico_City | Agente: Claude Code (Opus 5.5) | Tarea: WEB-005 ([#26](https://github.com/Codelab-ai-dev/brambiLab/issues/26)), PR 2 de 3: ejecución durable de publicaciones programadas
Rama: `feat/WEB-005-scheduler` | Commit base: `2900c53` (main, tras #27)

## Objetivo y aceptación
Ejecutar los trabajos programados dentro del proceso Go, sin Redis ni un worker aparte:
- con bloqueos de PostgreSQL;
- seguro con dos ejecutores y tras una caída;
- con backoff y un máximo de intentos, y con fallos terminales legibles;
- con reintento explícito y reloj inyectable;
- ejecutando en menos de 60 s tras la hora prevista.

## Cambios y archivos
- **`internal/publishing/runner.go`:**
  - **`Run`:** hace una pasada inmediata al arrancar y otra cada 15 s.
  - **`RunDue`:**
    - toma hasta 10 trabajos vencidos, primero los proyectos y después las bitácoras;
    - ejecuta cada uno en una transacción con el orden de bloqueo canónico;
    - revisa bajo el bloqueo que el trabajo siga programado y vencido.
  - **Fallos:**
    - **Terminales:** una validación o una ruta ocupada.
    - **Transitorios:** backoff de 30 s × 2ⁿ, con tope de 10 min y 5 intentos.
    - Cada resultado se registra en `publication_job_attempts`, sube `editorial_version` y deja auditoría con el actor `scheduler`.
  - **Bitácoras:** una bitácora cuyo proyecto tiene una programación pendiente espera sin gastar intentos.
  - **`Retry`:** revalida y devuelve el trabajo a la cola con la misma revisión y un presupuesto nuevo de intentos.
- **`handlers.go`:** `POST …/publication/jobs/{jobId}/retry`; 409 `job_not_failed`.
- **`cmd/server/main.go`:** arranca el ejecutor junto al servidor HTTP.
- **Pruebas y documentación:**
  - `export_test.go`: ganchos de prueba.
  - `scheduler_test.go`.
  - `web-v1.md` §8.1: orden proyecto → bitácora, espera del padre y semántica del reintento.
  - OpenAPI: descripción del reintento y del código 409.

## Verificaciones ejecutadas
- **`go vet ./...`, `gofmt` y `go test -count=1 ./...`** con PostgreSQL 18 desechable: todos `ok`.
- **`go test -race -count=5`, y después `-count=3` tras el último cambio**, en `./internal/publishing/`: `ok`.
- **Casos cubiertos:**
  - se publica la revisión congelada, no el borrador posterior, exactamente a la hora y ni un segundo antes;
  - proyecto y bitácora a la misma hora;
  - la bitácora espera al proyecto en backoff y falla si el proyecto se canceló;
  - un medio que pasa a privado da un fallo terminal; el reintento sigue bloqueado hasta corregirlo y luego se publica;
  - backoff exacto (30/60/120/240 s y después `failed`) y tope de 10 min;
  - caída antes del commit (contexto cancelado dentro de la transacción): sin rastro, y otro ejecutor lo recupera con un solo intento;
  - dos ejecutores: 8 trabajos, 8 éxitos, 8 intentos y 8 auditorías; con fallos, un solo intento por trabajo;
  - un trabajo en curso serializa una retirada, que después recibe 409 `editorial_conflict`;
  - una cancelación, retirada o publicación manual confirmada mientras el ejecutor espera el bloqueo gana, sin resurrección;
  - el bucle `Run` recupera al arrancar y respeta la hora.
- **Mutaciones manuales:** 15, de las que 13 fueron detectadas.
  - Una de las dos restantes era un control equivalente.
  - La otra, la guarda de `ctx.Err()`, es redundante porque el registro del fallo tampoco puede confirmarse con el contexto cancelado.
  - La comprobación de «vencido bajo bloqueo» sobrevivía al principio; la mata la nueva prueba con dos ejecutores fallando (3 de 3).
- **Redocly lint:** válido, con 7 avisos previos.
- **Compose local reconstruido:** la API arranca `healthy`, sin errores del ejecutor en los logs.
- **No ejecutado:**
  - e2e de programación por Caddy (PR 3);
  - CI (pendiente al abrir el PR).

## Pendientes y bloqueos
- **PR 3:**
  - estado editorial en el editor, la ficha y el historial;
  - acciones Publicar, Programar, Cancelar, Retirar y Reintentar, con confirmaciones y la zona America/Mexico_City visible;
  - no presentar el buffer sin guardar como publicado;
  - e2e por Caddy con cliente anónimo y medios reales.
- **Con los tres PR unidos:** cerrar #26 y actualizar el backlog.

## Decisiones propuestas o tomadas
- Un fallo transitorio no mantiene la transacción abierta: se deshace y se registra en otra, con el mismo orden de bloqueo.
- Una bitácora que espera a su proyecto no cuenta como intento. Se pospone al menos 30 s, para que un proyecto atascado no provoque un bucle.
- El reintento conserva el mismo trabajo: el historial de intentos queda en un solo lugar.

## Próxima acción concreta
Abrir el PR contra `main` con CI verde. Después, el panel con e2e del PR 3.

## Material para revisión
PR contra `main` (enlace en #26). Sin credenciales.
