# Plataforma web v1
Especificación: [web-v1.md](../docs/architecture/web-v1.md) · Decisión de stack: [ADR-006](../docs/architecture/ADR-006-web-stack.md).

| Ruta | Contenido |
|---|---|
| [web/](web/README.md) | React Router 8 (SSR, Node.js), TypeScript, Tailwind 4 |
| [api/](api/) | API Go: `serve`, `migrate` y `healthcheck` |
| [contracts/openapi.yaml](contracts/openapi.yaml) | Contrato de la API |
| [compose.yaml](compose.yaml) | postgres, migrate, api y web |

## Arranque local
Requiere Docker con Compose. Los puertos se publican sólo en `127.0.0.1`; PostgreSQL no se publica.
```bash
cd platform
cp .env.example .env        # valores locales; nunca subir .env
docker compose up --build -d --wait
curl localhost:8080/api/v1/health/ready
open http://localhost:3000
docker compose down         # conserva volúmenes; «down -v» los borra
```
Desarrollo sin contenedores:
```bash
cd platform/api && DATABASE_URL=postgres://… go run ./cmd/server migrate && DATABASE_URL=… go run ./cmd/server
cd platform/web && npm install && npm run dev   # proxy de /api y /media hacia :8080
```

## Verificación
```bash
cd platform/web && npm run typecheck && npm run build
cd platform/api && gofmt -l . && go vet ./... && go test ./...
TEST_DATABASE_URL=postgres://… go test -count=1 ./migrations/   # PostgreSQL desechable
npx -y @redocly/cli@2.54.3 lint platform/contracts/openapi.yaml
```
El mismo conjunto corre en [CI](../.github/workflows/ci.yml), junto con una prueba de humo con Compose.

## Decisiones de implementación (WEB-001)
Elecciones técnicas dentro del stack de ADR-006; se pueden revisar sin cambiar el ADR.
- API: `net/http` estándar con patrones de método; sin framework HTTP.
- PostgreSQL: pgx v5 con pool; conexión perezosa para que la API arranque y se reporte «no lista» mientras falte la base.
- Migraciones: goose v3 con SQL embebido, lock de sesión de PostgreSQL y servicio `migrate` de una sola ejecución antes de la API. Tabla base: `audit_events`.
- Imágenes: API en distroless `nonroot` (uid 65532) con healthcheck propio; web con el usuario `node`. `/data/media` pertenece a `nonroot`.
- PostgreSQL 18; su volumen se monta en `/var/lib/postgresql`, como requiere esa versión.
- Errores `{code,message,fields,request_id}`; `X-Request-ID` se acepta del proxy si es seguro y se genera si no.
- Web: `/` redirige temporalmente (302) a `/es`; `/:lang` sólo acepta `es` y `en`. Página provisional con `noindex` hasta WEB-006. Sin fuentes externas ni favicon hasta tener la identidad visual aprobada.

## Pendiente
Dominio y callback OAuth, recursos reales del VPS, datos de Resend y destino de backups (web-v1.md §17). `INTERNAL_API_URL` se declara en Compose, pero la web aún no llama a la API: la primera llamada SSR llega con WEB-002 o WEB-003.
