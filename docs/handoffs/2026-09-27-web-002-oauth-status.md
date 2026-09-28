# Handoff — Login sin configurar (seguimiento WEB-002)
Fecha: 2026-09-27 America/Mexico_City · Agente: Claude Code · Issue: #13
Rama: fix/WEB-002-oauth-status · Base: `7c0a459`

## Objetivo y aceptación
`/admin/login` no debe ofrecer el botón de GitHub cuando OAuth no está configurado, ni ocultarlo a visitantes anónimos cuando sí lo está. Un error de red no se confunde con falta de configuración. Respuestas privadas `no-store`.

## Cambios y archivos
- API: `GET /api/v1/auth/status` → `{"login_enabled": bool}`, público y `no-store`. Sólo expone ese booleano. OpenAPI 0.2.1.
- Web:
  - `lib/api.server.ts`: `getLoginAvailability()` (`enabled`, `not_configured` o `unavailable`). No envía cookies.
  - `lib/login.server.ts`: `loginState()` separa el estado de sesión de la disponibilidad.
  - `routes/admin/login.tsx`: tres estados visibles.
- e2e: comprueba la disponibilidad y el botón para visitantes anónimos con OAuth configurado.
- Web README y `tasks/current.md` (el punto de reanudación pasa a WEB-003 #12).

## Verificaciones ejecutadas
- **Reproducción antes del cambio** (stack aislado sin credenciales): `start` daba 503, pero `/admin/login` mostraba el botón y no el aviso.
- **Después, mismo stack:**
  - sin credenciales, `status` devuelve `{"login_enabled":false}` con `no-store`; no hay botón y aparece el aviso de «no configurado»;
  - con la API detenida, aparece «no responde», sin botón y sin el aviso de «no configurado».
- Go: `go vet` y `go test`, incluida `TestStatusReportsOnlyWhetherLoginIsEnabled`.
- Web: Vitest 11 pruebas (9 nuevas de `loginState`):
  - sin cookie, configurado, sin configurar, error de red y respuestas inesperadas;
  - cookies ajenas no reenviadas;
  - sesión válida redirige; sesión caducada muestra el login;
  - API caída;
  - `return_to` fuera de `/admin` se ignora.
  - Typecheck y build correctos.
- e2e con proxy real y OAuth configurado: 21 comprobaciones correctas.
- CI: con el PR.

## Pendientes y bloqueos
Ninguno. Nota operativa: con el stack local ocupando `:8000`, el e2e manual se ejecuta con `BASE=http://localhost:8200` y un `PUBLIC_HOST_PORT` distinto.

## Próxima acción concreta
Revisar y unir; continuar con WEB-003 (#12).
