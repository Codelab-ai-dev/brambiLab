# Handoff — Limpieza de la prueba de integración PostgreSQL
Fecha: 2026-09-27 America/Mexico_City · Agente: Claude Code · Issue: #8
Rama: fix/WEB-001-test-cleanup · Base: `844161d`

## Objetivo y aceptación
Que `TestReservedPasswordMigratesAndIsReady` no deje bases ni roles tras ejecutarse, también si falla a mitad de la preparación. Ejecutarla dos veces y comprobar que no queda nada.

## Cambios y archivos
`platform/api/internal/db/db_integration_test.go`:
- el cierre de la conexión admin se registra con `t.Cleanup` justo después de conectar, en lugar de `defer`, así que se ejecuta el último (LIFO);
- el rol y la base registran su limpieza justo después de crearse, y la base se borra antes que el rol;
- errores de limpieza con `t.Errorf` y timeout de 30 s;
- los errores de preparación no muestran el SQL, que contiene la contraseña.

Sin cambios de producción.

## Verificaciones ejecutadas
Contra PostgreSQL 18 desechable (`bl-test-pg`):
- **Antes del cambio:** cada ejecución dejaba 1 base y 1 rol `bl_reserved_*` (2 → 4 → 6 objetos). Reproducido.
- **Después:** dos ejecuciones seguidas, 0 objetos restantes. Las pruebas de contraseña reservada y readiness siguen pasando.
- `go vet` y `go test ./...` correctos.
- CI: se ejecuta con el PR.

## Pendientes y bloqueos
Ninguno. El helper `internal/testdb` de la rama WEB-002 recibe la misma corrección allí.

## Próxima acción concreta
Revisar y unir; luego continuar el PR #9 (WEB-002).
