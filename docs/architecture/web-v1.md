# BrambiLab Web v1 — Especificación técnica
Fecha: 2026-09-27 · Zona editorial: America/Mexico_City
Estado: requisitos acordados con Gustavo; diseño técnico listo para implementación, aún no implementado.
Responsable funcional: Gustavo. Decisión de stack: [ADR-006](ADR-006-web-stack.md).

## 1. Objetivo y alcance
Publicar el proceso de construcción de proyectos desde sus primeros experimentos. Inicio como portafolio profesional; fichas como laboratorio técnico. La web se construye ahora y no depende de terminar BL-001.
Incluye proyectos, bitácora por proyecto, artículos independientes, medios y recursos descargables, búsqueda y filtros, español/inglés, SEO y contacto.
Panel privado de un único administrador mediante GitHub. Visitantes sin registro, comentarios ni aportaciones.
Fuera de v1: pagos, comunidad, traducción automática, transcodificación de video, MCP, sincronización bidireccional Git/CMS y administración multiusuario.

## 2. Requisitos acordados
| Área | Decisión |
|---|---|
| Frontend | React + TypeScript; Tailwind CSS |
| Renderizado | SSR público mediante Node.js; API y negocio exclusivamente en Go |
| Datos | PostgreSQL |
| Infraestructura | Docker y Coolify; VPS Hostinger KVM 2 exclusivo |
| Edición | Editor visual e importación/exportación Markdown |
| Publicación | Borradores, vista previa, publicación manual y programada |
| Revisiones | Editar borrador conservando la versión pública hasta publicar |
| Idiomas | Interfaz i18n; contenido español/inglés traducido manualmente |
| Acceso | GitHub OAuth, restringido al ID del propietario |
| Medios | Archivos locales persistentes; interfaz preparada para S3 |
| Video | MP4 preparado por el autor, portada manual y enlaces YouTube |
| Descargas | Administrador decide qué archivos son públicos y descargables |
| Descubrimiento | Texto + categoría + etiqueta + tipo; idioma seleccionado |
| Contacto | Correo/LinkedIn/GitHub y formulario mediante Resend |
| Respaldos | Destino externo pendiente antes de lanzamiento |

## 3. Componentes y responsabilidades
Propuesta de implementación: React Router en modo framework con SSR para evitar crear un servidor de renderizado propio. Es una elección técnica de esta especificación; no cambia el stack acordado. Fijar versiones compatibles en lockfile y go.mod al iniciar; no se prescriben versiones no comprobadas.

```mermaid
flowchart TD
  U["Visitante o administrador"] --> C["Proxy HTTPS de Coolify (TLS)"]
  C --> P["Proxy de entrada · Caddy"]
  P --> W["React SSR · Node.js"]
  P --> A["API · Go"]
  W --> A
  A --> D["PostgreSQL"]
  A --> F["Volumen de archivos"]
  A --> E["GitHub OAuth y Resend"]
```

- web: páginas React públicas y panel /admin; consulta API interna para SSR. Sin credenciales PostgreSQL, reglas de publicación ni implementación paralela de autenticación.
- api: monolito modular Go: auth, content, publishing, media, search, contact. Único dueño de PostgreSQL, autorización y validación.
- postgres: datos editoriales, revisiones, sesiones y trabajos durables.
- Scheduler y envío pendiente: bucles en el proceso Go inicial, con trabajos en PostgreSQL, ejecución acotada y locks transaccionales. No Redis ni contenedor worker obligatorio en v1. Extraer worker si la carga lo justifica.
- Proxy de Coolify termina TLS y entrega todo el dominio al servicio proxy (Caddy, platform/proxy), única entrada pública: /api/v1/* y /media/* a Go; resto a web. Decisión de Gustavo del 2026-09-27 en WEB-002, frente al enrutado por ruta en Coolify, para que el mismo origen sea idéntico en local, en CI y en el VPS. PostgreSQL, API y web sin puerto público. SSR usa dirección privada http://api:8080.
- Una sola instancia de API al inicio; la cola debe tolerar reinicios y futuras instancias sin publicar dos veces.

## 4. Estructura de código prevista
- platform/web/: React, rutas públicas/admin, i18n, componentes, editor y Dockerfile.
- platform/api/: cmd/server, internal/{auth,content,publishing,media,search,contact,jobs}, migrations y Dockerfile.
- platform/compose.yaml: proxy, web, api, postgres y migrate; volúmenes y healthchecks. compose.e2e.yaml añade un GitHub simulado sólo para pruebas.
- platform/contracts/openapi.yaml: contrato versionado de API y tipos TS generables.
- docs/architecture/: decisiones y diseño.
Estas rutas describen entregables futuros; esta entrega no crea servicios ejecutables.

## 5. Fuentes de verdad
GitHub conserva código, especificaciones, decisiones, tareas y documentación de ingeniería reproducible.
PostgreSQL es la fuente de verdad del contenido editorial del panel; volumen local contiene bytes de medios. La web no lee documentos de Git en cada petición.
Importar Markdown es una acción explícita que crea/actualiza un borrador; exportar crea un snapshot portable. No sobrescribir publicaciones al hacer git push.
Los documentos de projects/ pueden respaldar artículos mediante enlaces; no mantener copias editorialmente sincronizadas a mano. No mover secretos o archivos privados a Git.
Desplegar código y publicar contenido son operaciones distintas.

## 6. Modelo de datos lógico
UUID internos; timestamps timestamptz UTC; FK, restricciones y migraciones SQL.
| Entidad | Campos y reglas principales |
|---|---|
| contents | id, kind project/article/log, project_id obligatorio sólo para log, created_at, archived_at |
| translations | id, content_id, locale es/en, published_revision_id nullable; UNIQUE(content_id,locale) |
| revisions | id, translation_id, version, title, slug, summary, body_json, body_schema_version, plain_text, seo, cover_asset_id, project_fields, created_at |
| revision_taxonomy | revision_id + category/tag; versionar relaciones para que editar filtros no cambie lo público prematuramente |
| categories/tags | identidad estable y etiquetas localizadas |
| revision_assets | revision_id, asset_id, uso embed/download; referencias de portada incluidas |
| publication_jobs | translation_id, revision_id, run_at, estado, intentos; publicación programada apunta a una revisión inmutable |
| assets | id, storage_backend, object_key, original_name, mime, size, sha256, estado, public_enabled, downloadable, created_at |
| asset_translations | asset_id, locale, alt_text, caption |
| sessions | hash del token, github_user_id, expiry, revoked_at |
| contact_messages/jobs | envío, estado, intentos, próxima ejecución, idempotency_key |
| slug_history | locale, ruta anterior y destino; redirección tras publicar cambio de slug |
| audit_events | actor, acción, entidad, fecha; nunca tokens ni cuerpo de mensajes |

Cada guardado genera revisión inmutable o snapshot versionado equivalente. published_revision_id sólo cambia al publicar. La API exige versión esperada/If-Match; conflicto devuelve 409 y conserva trabajo.
project_fields contiene objetivo, estado (idea/en desarrollo/pausado/completado), tecnologías, enlaces de repositorio y resultados. Su estado técnico es distinto del estado editorial.
Un log sólo es público si el proyecto padre tiene versión publicada en el mismo idioma. No permitir retirar un proyecto con logs públicos sin resolver explícitamente esa dependencia.
Unicidad del slug público por idioma y tipo/ruta; detectar conflictos también al ejecutar programación.
Retener revisiones en v1; su política de purga se definirá después. No borrar archivos todavía referenciados.

### 6.1 Implementación WEB-003
- `translations.latest_version` avanza de forma atómica con cada revisión. `published_revision_id` tiene una FK compuesta `(published_revision_id, id) → revisions(id, translation_id)`, así que no puede apuntar a una revisión de otra traducción. Ninguna operación de WEB-003 lo modifica; es de WEB-005.
- La revisión guarda el snapshot completo: título, slug, resumen, cuerpo, SEO, campos de proyecto, categoría (una, opcional) y etiquetas (varias). Las etiquetas se relacionan por revisión, así que editar la taxonomía no cambia versiones anteriores. Las categorías y etiquetas tienen una identidad estable y etiquetas es/en.
- Tipos de revisión: `manual`, `auto`, `restore` (con `restored_from_version`) y `copy` (borrador copiado de otro idioma, marcado como pendiente de traducir, nunca como traducción terminada).
- Archivar conserva el historial y se rechaza con 409 si alguna traducción está publicada. No hay borrado físico.
- Un log exige un proyecto padre existente, no archivado y de tipo project. El tipo y el padre no cambian después de crear el contenido.

### 6.2 Guardado y conflictos (decisión WEB-003)
Cada guardado exitoso, manual o automático, crea un snapshot inmutable completo.
- **Autoguardado:** tras 3 s de inactividad y sólo si hubo cambios; como máximo uno cada 15 s y una sola petición en vuelo por traducción. Los cambios hechos durante una petición quedan pendientes para el siguiente snapshot; una respuesta antigua no los marca como guardados. «Guardar revisión» persiste de inmediato y se serializa con los autoguardados pendientes.
- **Duplicados:** Go calcula un SHA-256 sobre el JSON canónico del snapshot. Si coincide con la última revisión, responde 200 `created: false` sin crear otra. La cabecera `Idempotency-Key` hace que el reintento de una misma operación devuelva la misma revisión.
- **Concurrencia:** `expected_version` es obligatorio. La comprobación y la inserción son atómicas (`UPDATE … WHERE latest_version = $expected`). Un conflicto responde 409 con la versión actual; el cliente conserva su buffer y ofrece recargar (tras confirmar el descarte) o exportar su trabajo local. Nunca reintenta con la versión del otro editor.
- **Estados visibles:** «Cambios sin guardar», «Guardando», «Guardado», «Error» y «Conflicto». Hay aviso al salir o al cambiar de idioma con cambios pendientes. Al recargar se recupera la última revisión guardada; no hay modo offline.
- **Restaurar** crea una revisión nueva (`restore`) y nunca modifica el original. El historial es paginado y distingue manual, automática, restauración y copia. Sin purga en v1.

## 7. Edición y Markdown
Documento estructurado JSON validado como formato canónico; no HTML arbitrario como fuente confiable. Implementación concreta del editor por seleccionar en WEB-003 con prueba de compatibilidad.
Bloques v1: títulos, párrafos, énfasis, listas, enlaces, citas, código con lenguaje, tablas simples, imágenes con alt, video MP4, YouTube y descarga de archivo.
Importación/exportación de Markdown del subconjunto soportado. Usar enlaces estables /media/{id} para medios; portabilidad completa requiere exportar los archivos junto al Markdown.
Embeds personalizados usan directivas documentadas (por ejemplo :::youtube, :::video, :::download) y esquema versionado. Advertir sobre sintaxis no soportada antes de importar; nunca perder bloques silenciosamente.
Cambiar de editor no debe descartar contenido. Importaciones remotas no descargan URLs arbitrarias desde el servidor. HTML/iframe libre deshabilitado; YouTube mediante ID/URL validada.
Pegar imágenes crea assets privados; texto alternativo y portada se editan en el panel.
Prueba de salida: visual → Markdown → importación conserva semántica de todos los bloques soportados. No prometer compatibilidad universal con cualquier Markdown.

### 7.1 Formato canónico v1 (WEB-003)
El formato canónico es un esquema JSON propio (`body_schema_version: 1`), no el JSON interno del editor. Un adaptador en el cliente convierte entre Tiptap y el formato canónico en ambos sentidos. Go valida de forma estricta y rechaza nodos, marcas o atributos desconocidos, aunque el documento haya pasado por el editor.
- **Bloques:** `paragraph`, `heading` (niveles 2 a 4; el título va aparte), `bulletList`, `orderedList` (`start`), `listItem`, `blockquote`, `codeBlock` (`language`, texto sin marcas), `horizontalRule`, `table`/`tableRow`/`tableHeader`/`tableCell` (tablas simples: cada celda es un párrafo, sin fusiones) y `youtube` (`videoId` de 11 caracteres validado, `start` opcional).
- **En línea:** `text` con las marcas `bold`, `italic`, `code` y `link` (`href`), y `hardBreak`.
- **Enlaces:** se aceptan `http`, `https`, `mailto`, rutas relativas `/…` y anclas `#…`. Se rechazan `javascript:`, `data:`, `vbscript:` y cualquier otro esquema. No hay HTML ni iframes.
- **Medios preparados para WEB-004:** `image` (`assetId`, `alt`, `caption`), `video` (`assetId`, `posterAssetId`, `caption`) y `download` (`assetId`, `label`). Desde WEB-004, la API sólo los acepta si el asset existe, está `ready` y es del tipo correcto (el póster y la portada, imágenes). Si no, responde 422 con el motivo. No se generan IDs ficticios.
- **Límites:** petición de 1 MiB como máximo, profundidad 32, 20 000 nodos y 200 000 caracteres de texto. Título de 200 caracteres, slug de 120 (`a-z0-9-`), resumen de 500, SEO de 70 y 160, y 20 etiquetas.

**Editor y Markdown (WEB-003, parte 2).**
- **Prueba previa (2026-09-27):** la extensión oficial `@tiptap/markdown` 3.31.3, en beta, no superó el fixture de prueba:
  1. no alargó el fence de un bloque de código que contenía ```` ``` ````, así que el documento quedó corrompido;
  2. exportó `\|` sin escapar dentro de una celda, con lo que la tabla ganó una columna;
  3. no entiende directivas;
  4. el JSON cambiaba en una segunda pasada.

  Por eso las conversiones son explícitas, sobre mdast (`mdast-util-from-markdown` y `mdast-util-to-markdown` con GFM tables y directivas, MIT), y trabajan con el formato canónico, no con Tiptap.
- **Dialecto:** CommonMark más tablas GFM, más directivas de hoja:
  - `::youtube{video=ID start=S}`
  - `::image{asset=UUID alt="…" caption="…"}`
  - `::video{asset=UUID poster=UUID caption="…"}`
  - `::download{asset=UUID label="…"}`

  Exportación con `-` en viñetas, `*` en énfasis, fences con ```` ` ```` (alargados cuando hace falta), escape de caracteres especiales y espacios de borde como `&#x20;`. Los párrafos vacíos, que sólo dan espaciado, no se exportan.
- **Importación:** nunca descarga URLs y nunca descarta contenido en silencio; todo lo no soportado se conserva como texto literal con aviso. En concreto:
  - HTML, directivas desconocidas y `texto:con:dos-puntos`;
  - el título 1 pasa a 2 y los títulos 5 y 6 pasan a 4;
  - las imágenes se convierten en enlace con su texto alternativo;
  - los enlaces inseguros quedan como texto;
  - se omiten la alineación de tablas y los metadatos de los bloques de código.
- **Editor:** Tiptap 3.31.3 sólo en cliente (`immediatelyRender: false`), restringido al esquema v1. Un adaptador convierte a canónico y normaliza de forma explícita: una celda con varias líneas se une con espacios y la primera fila de una tabla pasa a encabezado. Rechaza las celdas combinadas. Los fixtures compartidos (`platform/contracts/fixtures/documents`) atan Go y TypeScript al mismo contrato.

## 8. Publicación y programación
Por traducción: sin publicar, publicada, retirada. El borrador/revisión nueva puede coexistir con una publicación; la programación es un job separado.
1. Guardar crea revisión privada.
2. Vista previa requiere sesión del propietario, no-store y noindex.
3. Publicar valida contenido, slugs, medios y padre; cambia puntero público en una transacción.
4. Programar congela revision_id y convierte fecha local de America/Mexico_City a UTC.
5. Editar después no modifica la revisión programada: panel permite reemplazar o cancelar explícitamente.
6. Job reclama registros vencidos con bloqueo; publica atómicamente y registra resultado. Tras reinicio recupera vencidos; errores reintentables con backoff y límite, errores de validación visibles sin reintento infinito.
7. Retirar cancela jobs de esa traducción y excluye contenido de API pública, sitemap, búsqueda y rutas. Re-publicar requiere acción explícita.
Objetivo inicial: ejecutar dentro de 60 segundos del horario mientras servicio/DB estén saludables; mostrar retrasos/fallos en panel.
Primera versión sin caché compartida de HTML/contenido: no-store para evitar borradores expuestos y publicación obsoleta. Assets estáticos versionados sí pueden cachearse. Optimizar con invalidación más adelante.

### 8.1 Implementación WEB-005 (#26)
**Estado por traducción.**
- *Sin publicar*, *publicada* (`published_revision_id`) o *retirada* (`withdrawn_at` sin puntero).
- `editorial_version` es distinto de `latest_version` y avanza con publicar, programar, reemplazar, cancelar, retirar, reintentar y cada resultado de un trabajo. Todas esas acciones exigen `expected_editorial_version` y responden 409 `editorial_conflict` con el estado actual. Guardar un borrador no lo toca.
- Todas aceptan `Idempotency-Key`: la misma clave con el mismo cuerpo devuelve la respuesta original; con otro cuerpo, 422. Se guardan en `editorial_requests`, en la misma transacción.

**Publicar** una revisión guardada elegida explícitamente, en una transacción. Se valida:
- el contenido y la traducción existen y no están archivados;
- la revisión es de esa traducción, y el título y el slug son válidos;
- los medios referenciados están `ready` y `public_enabled`, y las descargas además `downloadable`;
- la portada y el póster son imágenes;
- una bitácora tiene su proyecto publicado, no archivado y en el mismo idioma.

Los permisos de los medios no se activan solos: el error 422 `publish_blocked` enumera los archivos afectados. Publicar actualiza el puntero, las fechas, la ruta y la auditoría, **y cancela cualquier programación pendiente** de esa traducción. Repetir una publicación idéntica no cambia nada. Si algo falla, la publicación anterior queda intacta.

**Rutas** (`public_routes`, que hace de *slug_history*):
- **Unicidad** `(locale, scope, slug)` con restricción de base de datos. El ámbito es `project` o `article` por idioma, o `log:<content_id del proyecto>`.
- **Guardar un borrador no reserva ruta.** Publicar la marca como vigente, y los slugs anteriores del mismo contenido quedan como alias reservados que otro contenido no puede usar.
- **Un alias resuelve a la ruta vigente de su contenido** en un solo salto: sin cadenas ni bucles.
- Las bitácoras dependen de la identidad del proyecto, no de su slug, así que un cambio de slug del proyecto no rompe sus rutas.
- Una traducción retirada conserva sus rutas reservadas, pero no responde ni redirige: 404.

**Visibilidad única.** La vista `visible_translations` (publicada, contenido no archivado y, si es bitácora, proyecto publicado y no archivado en el mismo idioma) la usan los medios, el lector público y los futuros listados, buscador y sitemap.

**Lector público mínimo**, con `no-store`, sólo con revisiones publicadas visibles:
- `GET /api/v1/public/{locale}/projects/{slug}`;
- `GET /api/v1/public/{locale}/articles/{slug}`;
- `GET /api/v1/public/{locale}/projects/{projectSlug}/logs/{slug}`.

Un alias responde 301 con `Location` hacia la ruta vigente. El sitio público completo es WEB-006.

**Programar.**
- La fecha y hora se introducen en America/Mexico_City y el servidor las convierte a UTC con la zona IANA (`time/tzdata` embebido). Se rechaza una fecha pasada.
- La revisión queda **congelada**; se valida al programar y otra vez al ejecutar.
- Si una bitácora se programa y su proyecto aún no está publicado, se acepta si el proyecto tiene una programación que vence antes o a la misma hora.
- Un solo trabajo activo por traducción (índice único parcial). Reemplazarlo es explícito (`replace: true`).
- Cada intento queda en `publication_job_attempts`.

**Ejecución** (en el proceso Go, sin Redis ni worker separado):
- Cada 15 s se toman como máximo 10 trabajos vencidos.
- Cada uno se ejecuta en **una transacción**, en este orden de bloqueo: traducción del proyecto padre (`FOR SHARE`) → traducción (`FOR UPDATE`) → trabajo (`FOR UPDATE`) → validación → publicación → trabajo `succeeded`.
- No existe estado `running`: los bloqueos de fila hacen de lease.
  - Si el proceso cae antes del commit, todo se deshace y el trabajo se recupera.
  - Si cae después, el trabajo consta como hecho y no se repite.
  - Un segundo ejecutor espera y encuentra el trabajo resuelto.
- **Errores transitorios** (base de datos): backoff de 30 s × 2ⁿ, con un tope de 10 min y 5 intentos, y después `failed`.
- **Errores de validación:** `failed` terminal con la causa legible.
- Primero los proyectos y después las bitácoras de la misma hora. Si una bitácora vence cuando su proyecto aún tiene una programación pendiente (por ejemplo, en backoff), espera a ese trabajo sin gastar intentos. Si el proyecto ya no tiene programación ni publicación, la bitácora falla como terminal.
- Reintentar es explícito y revalida, conservando la revisión elegida. El trabajo vuelve a la cola para la siguiente pasada con un presupuesto nuevo de intentos, y el registro de intentos continúa su numeración.
- Al arrancar, la primera pasada es inmediata: recupera lo que venció con el proceso caído.
- **Objetivo:** ejecutar en menos de 60 s tras la hora con la API y la base sanas. El reloj es inyectable en las pruebas.

**Cancelar y retirar.**
- Cancelar no retira lo que ya está publicado.
- Retirar quita el puntero y cancela los trabajos activos en la misma transacción, conservando revisiones y archivos.
- Un proyecto con bitácoras publicadas en ese idioma no se puede retirar (409 con la lista). No hay cascada y el otro idioma no se toca.
- Archivar se rechaza mientras haya publicación o programación activa.
- Todas las operaciones siguen el mismo orden de bloqueo, así que un trabajo que ya empezó termina antes o encuentra la cancelación: **no hay resurrección**.
- Los medios dejan de servirse de forma anónima cuando desaparece su última referencia publicada y visible. La autorización se comprueba antes de cualquier 304, HEAD o Range.

## 9. Rutas, idiomas, SEO y navegación
| Español | Inglés |
|---|---|
| /es | /en |
| /es/proyectos | /en/projects |
| /es/proyectos/:slug | /en/projects/:slug |
| /es/proyectos/:projectSlug/bitacora/:slug | /en/projects/:projectSlug/log/:slug |
| /es/articulos/:slug | /en/articles/:slug |
| /es/buscar | /en/search |
| /es/acerca-de y /es/contacto | /en/about y /en/contact |

/ redirige a /es como idioma por defecto inicial. Selector enlaza traducción publicada equivalente; si falta, indica que no está disponible y ofrece el índice del otro idioma, sin fingir traducción.
Admin /admin y login privado; interfaz inicial española con catálogo i18n preparado.
HTML SSR incluye título, texto, canonical absoluto, descripción, Open Graph, imagen pública y hreflang sólo de traducciones publicadas. Sitemap sólo con rutas canónicas públicas; robots/noindex para admin, previews y resultados de búsqueda.
Slugs editables por idioma; cambio publicado conserva redirect permanente evitando cadenas/bucles.
Diseño adaptable a móvil, navegación por teclado, contraste legible, alt de imágenes y formularios etiquetados.
Homepage: presentación, proyectos destacados, últimos avances, artículos y contacto. Panel administra también bio, enlaces y selección de destacados.
La identidad visual utiliza assets aprobados por Gustavo; no inventar un logo definitivo ni sustituirlo por un render.
**Dirección visual «cuaderno de laboratorio»** (elegida por Gustavo el 2026-09-27):
- Fondo papel y texto tinta; ámbar como señal (enlaces, foco, estado activo) y tinta para las acciones principales. Modo oscuro en grafito con ámbar encendido.
- IBM Plex Sans para leer e IBM Plex Mono para datos (fechas, versiones, identificadores), incluidas en el propio sitio (OFL).
- Retícula milimetrada decorativa sólo en portada y login.
- Logotipo sólo tipográfico («BRAMBILAB_»); no hay logo gráfico hasta que existan assets aprobados.
- Tokens semánticos en `platform/web/app/app.css`: contraste AA en texto y 3:1 en bordes de controles, comprobados par a par.
- **Portada «banco de pruebas vivo»** (elegida por Gustavo el 2026-09-28): hero y cabecera pública siempre oscuros, como un instrumento, con las secciones de debajo en papel.
  - Osciloscopio SVG cuya traza pasa de senoidal a cuadrada (analógico → digital).
  - Las 5 áreas como chips conectados por pistas con un pulso; retícula que se ilumina bajo el cursor.
  - Sin JavaScript o con `prefers-reduced-motion`, todo queda estático y completo. Lo decorativo es `aria-hidden`.
  - Sólo contenido real del repositorio.

## 10. API REST inicial
Prefijo /api/v1; JSON, paginación y errores {code,message,fields,request_id}. Contrato en OpenAPI.
| Grupo | Operaciones previstas |
|---|---|
| Público | GET /projects, /articles, /logs, /search, /site; filtros locale, category, tag, type y paginación |
| Detalle | GET /content/{kind}/{slug}?locale=es; sólo revisión publicada y padre visible |
| Autenticación | GET /auth/github/start, /auth/github/callback, /auth/me; POST /auth/logout |
| Administración | CRUD /admin/contents y /admin/contents/{id}/translations/{locale}/revisions |
| Publicación | POST publish/schedule/withdraw sobre traducción; DELETE schedule; GET revisions y preview |
| Medios | POST /admin/assets; PATCH metadatos/permisos; DELETE sólo sin referencias; GET /media/{id} y /media/{id}/download |
| Contacto | POST /contact, sin archivos adjuntos; respuesta 202 cuando se guarda para envío |
| Operación | GET /health/live y /health/ready |

Rutas exactas se congelan al crear OpenAPI antes de implementar frontend/backend en paralelo. Ninguna operación admin se autoriza por ocultar botones.
Búsqueda propuesta: PostgreSQL full-text con configuración español/inglés y GIN; índice derivado exclusivamente de revisión publicada. Actualizar en transacción de publicación/retirada. Sin Elasticsearch en v1.
Pruebas incluyen filtros combinados, idioma y ausencia de títulos/fragmentos de borradores.

## 11. GitHub OAuth y sesiones
Go inicia flujo Authorization Code, state aleatorio de un solo uso y PKCE; callback fijo y validado. Tras identidad GitHub, comparar ID numérico con ADMIN_GITHUB_USER_ID, nunca sólo nombre o correo. No crear usuarios por aceptación de OAuth.
No solicitar permisos de repositorios: autenticación de identidad solamente. No exponer client secret/token al navegador.
Sesión propia opaca, token aleatorio hasheado en DB; cookie Secure, HttpOnly, SameSite=Lax y Path=/. Expiración y revocación, logout servidor. Propuesta de duración: 12 horas, configurable.
CSRF y validación Origin para mutaciones; CORS mismo origen por defecto. Sanitización/validación de contenido, límites y consultas parametrizadas.
Secretos exclusivamente en Coolify; .env.example sin valores reales.

## 12. Archivos, video y migración S3
Interfaz Go Storage: Put, Open/ReadRange, Stat, Delete. Backend local primero; key opaca sin rutas basadas en nombre enviado. DB separa provider/key de URL pública estable.
Volumen /data/media separado de imagen Docker. Subida streaming a temporal, límite de bytes, comprobación MIME/firma y escritura final atómica; estados pending/ready/failed. Limpiar temporales huérfanos.
Regla pública: asset ready + public_enabled + referencia desde revisión publicada visible. downloadable exige además bandera explícita. Sin publicación, sólo acceso autenticado. Retirar contenido elimina acceso si no queda otra referencia pública.
Go sirve medios comprobando reglas; volumen no se expone como directorio estático. Soportar HEAD y Range/206 para MP4 y 416 en rangos inválidos. Descargar documentos con Content-Disposition attachment y nosniff.
Limites iniciales propuestos y configurables: imagen 20 MiB, documento/recurso 100 MiB, MP4 250 MiB; 1 subida grande concurrente. Alinear límites/timeouts de proxy y API; comprobar espacio antes de aceptar.
MP4 con codecs preparados compatibles con navegadores objetivo; no asumir que toda extensión .mp4 reproduce. Sin FFmpeg, HLS ni conversiones de video automáticas en v1. Portada manual y YouTube con dominio/ID permitido; carga del embed al interactuar.
Imágenes: quitar EXIF antes de hacerlas públicas y controlar dimensiones; SVG/HTML ejecutables no se muestran inline. Recursos ZIP/STL se entregan como descarga; no descomprimirlos en servidor.
Migración futura: copiar bytes a S3, verificar hash/tamaño, cambiar provider/key por lotes y conservar volumen anterior hasta validar. No cambiar URLs /media/{id}; adaptador decide stream o entrega autorizada. Ningún bucket público para borradores.
Límite de disco y alertas por ocupación; seleccionar umbral al verificar capacidad real.

### 12.1 Decisiones de implementación (WEB-004, #20)
**Formatos admitidos.** Se validan por su firma y estructura, nunca por el `Content-Type` del cliente; la extensión sólo orienta.

| Tipo | Formatos | Límite | Entrega |
|---|---|---|---|
| Imagen | JPEG, PNG, WebP | 20 MiB, 16 384 px por lado, 50 MP | en línea (`/media/{id}`) |
| Vídeo | MP4 (ISO BMFF: `ftyp` + `moov`) | 250 MiB | en línea con Range/206 |
| Recurso | PDF, ZIP, STL (ASCII o binario) | 100 MiB | sólo `attachment` (`/media/{id}/download`) |

- **No admitidos**, con un mensaje claro: SVG (puede ejecutar código), GIF, HEIC/HEIF (no hay decodificador en Go; hay que exportar a JPEG), AVIF, ejecutables y cualquier otro formato.
- Los ZIP no se descomprimen ni se inspeccionan. Validar el contenedor MP4 no demuestra que los códecs sean compatibles. Perfil recomendado: H.264 (High o Main) + AAC, `faststart` (moov antes de mdat), 1080p como máximo.

**Imágenes.**
- **JPEG:** se eliminan APP1 (EXIF, XMP), APP13 (IPTC) y COM sin recomprimir, conservando el perfil ICC. Si la orientación EXIF no es 1, se rota de verdad, se recodifica con calidad 92 y se reinserta el ICC.
- **PNG:** se eliminan `eXIf`, `tEXt`, `zTXt`, `iTXt` y `tIME`.
- **WebP:** se eliminan los chunks `EXIF` y `XMP ` y se corrigen las banderas de VP8X. Una WebP con orientación distinta de 1 se rechaza, porque no hay codificador WebP en Go.
- Antes de decodificar se comprueban las dimensiones (`DecodeConfig`), como protección frente a bombas de descompresión.
- El hash y el tamaño guardados son los de los bytes finales. El ancho y el alto se guardan para reservar espacio al renderizar.

**Subida.**
- `POST /api/v1/admin/assets?filename=…` con el archivo como cuerpo de la petición, sin multipart. `Content-Length` es obligatorio (si falta, 411).
- **Antes de leer:**
  - se comprueban el límite (413) y el espacio libre del volumen, dejando un margen de 512 MiB (si no alcanza, 507);
  - sólo se admite una subida grande (más de 20 MiB) a la vez; si hay otra en curso, 429 con `Retry-After`.
- **Proceso:**
  1. se lee en streaming a `/data/media/tmp` con el límite aplicado;
  2. se valida y se procesa;
  3. se mueve con un rename atómico a una key opaca `ab/cd/<32 hex>`.
- **Estados:** la fila empieza como `pending` y sólo pasa a `ready` si el archivo final existe y su tamaño coincide. Cualquier error la deja en `failed` y borra lo escrito. Un disco lleno durante la escritura también da 507.
- **Limpieza:** un proceso borra los temporales sin actividad durante más de 2 horas (salvo las subidas activas) y marca como `failed`, borrando sus bytes, las filas `pending` de más de 2 horas.
- Caddy limita el cuerpo de `/api/v1/admin/assets` a 256 MiB.

**Modelo.**
- `assets`: `storage_backend`, `object_key` opaca, nombre original saneado, MIME, bytes, SHA-256, dimensiones, `status`, `public_enabled` y `downloadable`. Las subidas nuevas son privadas y no descargables.
- `asset_translations`: alt y caption en es/en como **valores por defecto de la biblioteca**. Los nodos del documento guardan su propio alt y caption en el snapshot, así que editar la biblioteca nunca cambia revisiones anteriores.
- `revision_assets (revision_id, asset_id, usage)`, con usage `image`, `video`, `poster`, `download` o `cover`, se inserta en la misma transacción que la revisión. `ON DELETE RESTRICT`, así que un asset referenciado por cualquier revisión retenida no se puede borrar. El guardado bloquea las filas de los assets con `FOR KEY SHARE`, lo que serializa el guardado frente a un borrado concurrente.
- Los bytes son inmutables: reemplazar un archivo crea un ID nuevo.
- El snapshot incorpora `cover_asset_id` (imagen `ready`, opcional).

**Entrega y autorización** (`GET`/`HEAD` en `/media/{id}` y `/media/{id}/download`, comprobadas en cada petición):
- **Propietario con sesión:** cualquier asset `ready`, con `Cache-Control: private, no-store`.
- **Anónimo:** sólo si el asset está `ready` y `public_enabled`, y hay al menos una referencia desde una revisión **publicada** de un contenido no archivado. Si es una bitácora, además su proyecto tiene que estar publicado en el mismo idioma. La descarga exige también `downloadable`. En cualquier otro caso, 404 idéntico y sin metadatos, también en HEAD y Range.
- Los recursos (PDF, ZIP, STL) nunca se sirven en línea: `/media/{id}` responde 404 y sólo existe `/download`.
- **Cabeceras:** `X-Content-Type-Options: nosniff`, `Content-Security-Policy: default-src 'none'; sandbox`, `Content-Disposition` con `filename*` UTF-8 saneado y MIME validado.
- **Rangos:** `http.ServeContent` resuelve HEAD, 206, `Content-Range`, `Accept-Ranges`, 416 para rangos no satisfacibles y malformados, y rangos múltiples como `multipart/byteranges`. La memoria queda acotada porque se lee del archivo.
- **Caché:** los medios públicos llevan `Cache-Control: no-cache` + ETag (SHA-256), así que se revalidan en cada petición y dejan de entregarse en cuanto se retiran o revocan. Sin caché pública duradera ni CDN.

## 13. Contacto y Resend
Enlaces configurables a correo, LinkedIn y GitHub. Formulario nombre, correo y mensaje, sin adjuntos.
Go valida tamaños/formato, aplica honeypot y rate limit; guarda job antes de responder 202. Remitente desde dominio verificado en Resend; correo del visitante sólo Reply-To; destinatario fijo configurable, nunca arbitrario.
Reintentos limitados con backoff e idempotency key estable. Panel muestra pendientes/fallidos y permite reintento consciente; no indicar «entregado» sólo por aceptación de API.
Evitar guardar el texto completo en logs. Retención propuesta de mensajes: 30 días; informar uso del formulario en aviso de privacidad.
Antes de habilitar: dominio remitente, destinatario y API key configurados; sin configuración, formulario deshabilitado con enlaces alternativos. No enviar correos reales como parte de esta especificación.

## 14. Docker y Coolify
Cuatro servicios runtime: proxy, web, api y postgres, más la tarea migrate. Build multietapa, usuario no-root donde corresponda, imágenes versionadas y dependencias fijadas; sin montar código fuente en producción.
Volúmenes postgres_data y media_data persistentes. Red interna para DB/API; exposición pública sólo por proxy. Healthchecks y reintentos de conexión; readiness comprueba dependencias sin filtrar credenciales.
Migraciones como tarea de release antes de activar nueva API; un ejecutor con lock. Cambios compatibles hacia adelante; no ejecutar rollback destructivo automáticamente.
Configuración: PUBLIC_ORIGIN, INTERNAL_API_URL, DATABASE_URL, ADMIN_GITHUB_USER_ID, GITHUB_CLIENT_ID/SECRET, SESSION_SECRET si se requiere firma, RESEND_API_KEY, CONTACT_FROM/TO, STORAGE_DRIVER, STORAGE_LOCAL_ROOT, upload limits y TZ editorial.
Dominio final y callbacks pendientes. No asumir capacidad exacta de KVM 2: verificar RAM/CPU/disco en panel antes de fijar límites. Ajustar pool DB, concurrencia y memoria mediante prueba de carga; no prometer tráfico soportado.
CI prevista: lint/typecheck/build frontend; go vet/test/build; integración PostgreSQL; migraciones; construcción Docker. Despliegue mediante Coolify desde rama acordada con autorización, no despliegue automático en esta entrega.

## 15. Operación y respaldos
Logs estructurados con request_id; métricas de latencia, errores, jobs atrasados, disco y fallos de contacto. No registrar tokens, cuerpos privados ni datos sensibles innecesarios.
Backups: PostgreSQL + bytes media + configuración necesaria, cifrados fuera del VPS. Destino, frecuencia, retención y objetivos RPO/RTO pendientes de elección antes de lanzamiento.
Diseñar copia consistente: congelar borrados/cambios destructivos o snapshot coordinado, registrar manifiesto de objetos y verificar referencias DB. Un volumen persistente no es un backup.
Gate de lanzamiento: restaurar DB y archivos en entorno separado y comprobar login, referencias y publicaciones. Proveedor pendiente no bloquea desarrollo.

## 16. Entregas y aceptación
| Tarea | Resultado verificable |
|---|---|
| WEB-001 | Scaffold React SSR/Go/Postgres, Compose, healthchecks, OpenAPI base, migraciones reproducibles |
| WEB-002 | OAuth sólo propietario; sesión/cierre; otra cuenta rechazada; CSRF probado |
| WEB-003 | Proyectos/artículos/logs, traducciones, revisiones y editor; round-trip Markdown probado |
| WEB-004 | Medios locales privados/públicos, descargas y MP4 con Range; límites y EXIF verificados |
| WEB-005 | Publicar/programar/retirar; job sobrevive reinicio; revisión nueva no altera contenido público |
| WEB-006 | Portafolio, fichas, bitácoras, i18n, SSR, SEO, búsqueda y filtros sólo públicos |
| WEB-007 | Contacto Resend con proveedor simulado en tests; configuración real y envío de prueba autorizado |
| WEB-008 | Coolify, configuración final, backup/restauración y prueba integral previa al lanzamiento |

Pruebas críticas: acceder sin sesión a preview/asset privado devuelve rechazo; cuenta GitHub distinta rechazada; dos ediciones detectan conflicto; programación publica revisión congelada; retirada elimina búsqueda/sitemap/medios sin otras referencias; HTML sin JS contiene contenido y metadatos; traducción faltante no produce página ficticia; redespliegue preserva DB/archivos; descargas y video no cargan archivos completos en RAM.
Orden: WEB-001 → WEB-002 → WEB-003 → WEB-004 → WEB-005 → WEB-006 → WEB-007 → WEB-008. Sin condicionar web a movimiento del rover.

## 17. Contenido inicial y pendientes
BL-001 servirá como primer proyecto en desarrollo. Datos reportados en conversación: cámara funcionó alimentada por USB; dos baterías de 3.7 V disponibles sin prueba con ellas; motores sin prueba reportada. Identificación visual del driver probable L298N, sin referencia del chip confirmada. No convertirlos en mediciones verificadas ni declarar inventario cerrado.
Esta entrega no modifica las fichas físicas ni publica archivos del fabricante. Importación editorial posterior con referencia al origen y revisión de recursos.

Pendientes operativos: dominio/callback OAuth, ID administrador configurado, recursos exactos del VPS, datos Resend, destino/política de backup. Pendientes de implementación: editor y formato de directivas final, versiones de dependencias y calibración de límites. Ninguno debe presentarse como servicio ya desplegado.

## 18. Referencias técnicas
Consultadas 2026-09-27; son soporte de las elecciones, no pruebas de implementación:
- [React Router: estrategias de renderizado](https://reactrouter.com/start/framework/rendering): SSR como modo soportado para React.
- [GitHub OAuth](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps): Authorization Code, state y PKCE.
- [Resend: idempotency keys](https://resend.com/docs/dashboard/emails/idempotency-keys): controlar reintentos de envío.
