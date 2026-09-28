# BrambiLab Web v1
Estado: scaffold de WEB-001. SSR con React Router 8, TypeScript y Tailwind 4. Sin contenido publicado, panel ni autenticación todavía.
- [Especificación técnica](../../docs/architecture/web-v1.md)
- [ADR-006](../../docs/architecture/ADR-006-web-stack.md)
- [Plataforma: arranque y verificación](../README.md)

| Ruta | Comportamiento |
|---|---|
| `/` | 302 a `/es` |
| `/es`, `/en` | Página provisional, `noindex` |
| otro `/:lang` | 404 |
| `/healthz` | Liveness del proceso SSR |

Comandos: `npm run dev` (proxy de `/api` y `/media` a `API_PROXY_TARGET`, por defecto `http://localhost:8080`), `npm run typecheck`, `npm run build`, `npm run start`.
Contenido editorial en PostgreSQL mediante la API Go; la web no lee Markdown de Git ni accede a la base de datos.
