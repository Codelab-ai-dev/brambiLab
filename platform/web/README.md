# BrambiLab Web v1
Estado: WEB-001 y WEB-002. SSR con React Router 8, TypeScript y Tailwind 4. Acceso del propietario al panel; todavía sin contenido publicado.
- [Especificación técnica](../../docs/architecture/web-v1.md)
- [ADR-006](../../docs/architecture/ADR-006-web-stack.md)
- [Plataforma: arranque y verificación](../README.md)

| Ruta | Comportamiento |
|---|---|
| `/` | 302 a `/es` |
| `/es`, `/en` | Página provisional, `noindex` |
| otro `/:lang` | 404 |
| `/healthz` | Liveness del proceso SSR |
| `/admin/login` | Botón «Iniciar sesión con GitHub» sólo si Go informa que el login está habilitado (`/api/v1/auth/status`); si no, aviso de «no configurado» o de «no responde». Mensajes de rechazo; `no-store` |
| `/admin` | Requiere sesión (la verifica Go en cada petición); sin ella, redirige a login |
| `/admin/contenidos?tipo=…` | Listado de proyectos, artículos o bitácora; archivados aparte |
| `/admin/contenidos/nuevo` | Crear (idioma inicial; proyecto para una bitácora) |
| `/admin/contenidos/:id` | Ficha: traducciones, archivado y bitácora del proyecto |
| `/admin/contenidos/:id/:locale` | Editor con autoguardado, conflictos e importación/exportación de Markdown |
| `/admin/contenidos/:id/:locale/historial` | Revisiones paginadas; restaurar crea una versión nueva |
| `/admin/contenidos/:id/:locale/v/:n` | Vista previa privada de una revisión guardada |
| `/admin/taxonomia` | Categorías y etiquetas (es/en) |

Comandos: `npm test` (Vitest), `npm run e2e` (Playwright contra el stack e2e), `npm run dev` (proxy de `/api` y `/media` a `API_PROXY_TARGET`, por defecto `http://localhost:8080`), `npm run typecheck`, `npm run build`, `npm run start`.
Contenido editorial en PostgreSQL mediante la API Go; la web no lee Markdown de Git ni accede a la base de datos.
