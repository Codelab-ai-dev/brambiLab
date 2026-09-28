# Plataforma web v1
Especificación: [web-v1.md](../docs/architecture/web-v1.md) · Decisión de stack: [ADR-006](../docs/architecture/ADR-006-web-stack.md).

| Ruta | Contenido |
|---|---|
| [web/](web/README.md) | React Router 8 (SSR, Node.js), TypeScript, Tailwind 4 |
| [api/](api/) | API Go: `serve`, `migrate` y `healthcheck` |
| [contracts/openapi.yaml](contracts/openapi.yaml) | Contrato de la API |
| [proxy/](proxy/Caddyfile) | Caddy: única entrada pública; `/api/v1/*` y `/media/*` → api, resto → web |
| [compose.yaml](compose.yaml) | proxy, web, api, postgres y migrate |
| [compose.e2e.yaml](compose.e2e.yaml), [e2e/](e2e/auth.sh) | GitHub simulado y prueba de login de punta a punta (sólo pruebas) |

## Arranque local
Requiere Docker con Compose. Sólo el proxy publica un puerto (`127.0.0.1:8000`); web, API y PostgreSQL quedan en redes internas.
```bash
cd platform
cp .env.example .env        # valores locales; nunca subir .env
docker compose up --build -d --wait
curl localhost:8000/api/v1/health/ready
open http://localhost:8000
docker compose down         # conserva volúmenes; «down -v» los borra
```
Desarrollo sin contenedores:
```bash
export PGHOST=localhost PGUSER=… PGPASSWORD=… PGDATABASE=… PGSSLMODE=disable
cd platform/api && go run ./cmd/server migrate && go run ./cmd/server
cd platform/web && npm install && npm run dev   # proxy de /api y /media hacia :8080
```

## Verificación
```bash
cd platform/web && npm run typecheck && npm run build
cd platform/api && gofmt -l . && go vet ./... && go test ./...
TEST_DATABASE_URL=postgres://… go test -count=1 ./migrations/   # PostgreSQL desechable
npx -y @redocly/cli@2.54.3 lint platform/contracts/openapi.yaml
```
Login de punta a punta, a través del proxy real y con un GitHub simulado:
```bash
docker compose -f compose.yaml -f compose.e2e.yaml up --build -d --wait
./e2e/auth.sh
docker compose -f compose.yaml -f compose.e2e.yaml down -v
```
Si ya tienes el stack local en `:8000`, usa un proyecto y un puerto aparte para no probar contra él: `-p bl-e2e`, con `PUBLIC_HOST_PORT=8200` y `PUBLIC_ORIGIN=http://localhost:8200` en un env-file propio, y `BASE=http://localhost:8200 ./e2e/auth.sh`.
Todo este conjunto corre también en [CI](../.github/workflows/ci.yml).

## Acceso del propietario (WEB-002)
1. Crea una OAuth App en GitHub (*Settings → Developer settings → OAuth Apps*) con homepage `PUBLIC_ORIGIN` y callback `PUBLIC_ORIGIN/api/v1/auth/github/callback`. Para producción hace falta otra app, porque GitHub sólo admite un callback por app.
2. En `.env` (o en los secretos de Coolify): `PUBLIC_ORIGIN`, `ADMIN_GITHUB_USER_ID` (ID numérico, no el usuario), `GITHUB_CLIENT_ID` y `GITHUB_CLIENT_SECRET`. Sin ellos el login queda deshabilitado y la API responde 503 en `/api/v1/auth/*`.
3. Entra en `PUBLIC_ORIGIN/admin`.

Seguridad:
- Authorization Code con PKCE S256 y sin scopes. El `state` es de un solo uso, se guarda con hash y va ligado al navegador por una cookie.
- Sólo el ID numérico del propietario recibe sesión; cualquier otra cuenta se rechaza y queda auditada. El token de GitHub nunca se guarda.
- La sesión es opaca y se guarda con hash. Va en una cookie `HttpOnly`, `SameSite=Lax`, `Path=/` que dura 12 h (`SESSION_TTL`). Con https es `__Host-` y `Secure`; http sólo se acepta en localhost.
- Las mutaciones exigen un `Origin` igual a `PUBLIC_ORIGIN` y el token CSRF de la sesión. Las respuestas privadas son `no-store`.
- El SSR de `/admin` reenvía sólo la cookie de sesión y sólo a `INTERNAL_API_URL`. La web no decide nada: Go autentica y autoriza.

## Decisiones de implementación (WEB-001)
Elecciones técnicas dentro del stack de ADR-006; se pueden revisar sin cambiar el ADR.
- API: `net/http` estándar con patrones de método; sin framework HTTP.
- PostgreSQL: pgx v5 con pool; conexión perezosa para que la API arranque y se reporte «no lista» mientras falte la base. La conexión se configura con las variables estándar `PG*`, leídas tal cual, sin construir URLs; así la contraseña admite caracteres reservados. `DATABASE_URL` sigue aceptándose si ya está bien codificada, y sus errores nunca repiten la cadena.
- Migraciones: goose v3 con SQL embebido, lock de sesión de PostgreSQL y servicio `migrate` de una sola ejecución antes de la API. Tabla base: `audit_events`.
- Imágenes: API en distroless `nonroot` (uid 65532) con healthcheck propio; web con el usuario `node`. `/data/media` pertenece a `nonroot`.
- PostgreSQL 18; su volumen se monta en `/var/lib/postgresql`, como requiere esa versión.
- Errores `{code,message,fields,request_id}`; `X-Request-ID` se acepta del proxy si es seguro y se genera si no.
- Web: `/` redirige temporalmente (302) a `/es`; `/:lang` sólo acepta `es` y `en`. Página provisional con `noindex` hasta WEB-006. Sin fuentes externas ni favicon hasta tener la identidad visual aprobada.

## Contenido editorial (WEB-003, en curso: #12)
API privada en `/api/v1/admin/contents`, además de `categories` y `tags` ([contrato](contracts/openapi.yaml)):
- Proyectos, artículos y bitácoras en es/en, con revisiones inmutables.
- `expected_version` obligatorio y 409 ante conflictos.
- Snapshots idénticos no crean revisión nueva; `Idempotency-Key` para reintentos.
- Restaurar crea una revisión nueva.

El documento usa el formato canónico v1 y Go lo valida de forma estricta (web-v1.md §7.1). PostgreSQL también impide cambiar el tipo o el padre, reescribir revisiones y publicar una revisión de otra traducción. Panel en `/admin` (sólo en español por ahora):
- listados por tipo y fichas con traducciones es/en (vacía o copia marcada) y bitácora del proyecto;
- editor Tiptap con los datos del contenido, autoguardado (3 s de inactividad, 15 s como mínimo entre guardados) y guardado manual;
- diálogo de conflicto que conserva lo escrito;
- importación de Markdown con avisos y confirmación, y exportación;
- historial con restauración y vista previa privada;
- archivado.

Las mutaciones van del navegador a Go con el token CSRF; las lecturas SSR reenvían sólo la cookie. Caddy fuerza `no-store` y `noindex` en todo `/admin*`.

E2E del panel en un navegador (Playwright + Chromium), con el stack de `compose.e2e.yaml` en marcha:
```bash
cd platform/web && npx playwright install chromium
BASE_URL=http://localhost:8000 npm run e2e      # SHOTS_DIR=/tmp/x guarda capturas
```

## Medios (WEB-004, #20)
**Biblioteca** en `/admin/medios`:
- subida con progreso, cancelación y reintento;
- textos es/en por defecto, permisos y lista de dónde se usa cada archivo;
- borrado sólo si ninguna revisión lo usa.

**Editor:** insertar imagen, vídeo (con póster) y descarga desde la biblioteca, elegir portada, y pegar o soltar imágenes.

**Validación y saneado** (web-v1.md §12.1):
- JPEG, PNG y WebP (sin EXIF/GPS/XMP y con la orientación aplicada), MP4, y PDF/ZIP/STL;
- archivos disfrazados, SVG, HEIC y ejecutables rechazados.

**Entrega** en `/media/{id}` y `/media/{id}/download`:
- autorizada en cada petición: el propietario ve todo; un visitante, sólo lo público y referenciado por una revisión publicada;
- Range/206/416, `nosniff`, CSP `sandbox`, `no-cache` + ETag.

Volumen `media_data` → `/data/media` (`STORAGE_LOCAL_ROOT`); límites en `MEDIA_MAX_*_BYTES`.

Dominio y callback OAuth, recursos reales del VPS, datos de Resend y destino de backups (web-v1.md §17). Revocar el token de GitHub tras leer la identidad (hoy sólo se descarta) y limitar la tasa de `/auth/github/start`: mejoras propuestas, no implementadas.
