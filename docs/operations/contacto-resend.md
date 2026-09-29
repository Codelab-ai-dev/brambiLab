# Activar el formulario de contacto con Resend

Estado: **preparado y desactivado.** El código de WEB-007 está listo, pero el formulario no acepta mensajes y no se envía ningún correo hasta que completes los pasos de esta guía. No se ha hecho ningún envío real a Resend: todas las pruebas usan un simulador local.

Referencia técnica: [web-v1.md §13.1](../architecture/web-v1.md). La API oficial se consultó el 2026-09-28 y conviene volver a revisarla antes de activar:
- [Enviar un correo](https://resend.com/docs/api-reference/emails/send-email)
- [Claves de idempotencia](https://resend.com/docs/dashboard/emails/idempotency-keys)

## 1. En Resend (tu cuenta)
1. **Dominio remitente:** añade en *Domains* el dominio (o subdominio) desde el que saldrán los avisos, por ejemplo `brambilab.example`.
2. **DNS:** crea en tu proveedor los registros que muestra Resend (SPF, DKIM y, si lo pide, MX o return-path). Espera hasta que el dominio aparezca como **Verified**. Sin dominio verificado, Resend rechaza el envío (403 `validation_error`) y el mensaje queda como `failed`.
3. **Clave de API:** créala en *API Keys* con permiso **Sending access** y, si puedes, limitada a ese dominio. Cópiala una sola vez.
   - No la pegues en issues, chats, commits ni conversaciones con agentes.

## 2. Valores que necesitas
| Variable | Ejemplo | Notas |
|---|---|---|
| `CONTACT_ENABLED` | `true` | Cualquier otro valor lo desactiva. |
| `RESEND_API_KEY` | `re_…` | **Secreto.** Sólo en Coolify. |
| `CONTACT_FROM` | `BrambiLab <contacto@brambilab.example>` | Debe ser del dominio verificado. |
| `CONTACT_TO` | `tu-buzon@ejemplo.com` | Destinatario fijo. No cambia con el correo público del sitio. |

Opcionales, con valores iniciales no medidos:
- `CONTACT_RATE_PER_CLIENT`: 5 cada 15 minutos.
- `CONTACT_RATE_GLOBAL`: 100 aceptados por hora.
- `CONTACT_RETENTION_DAYS`: 30.
- `TRUSTED_PROXIES`: rangos privados por defecto.

## 3. En Coolify
1. En el servicio **api** (no en `web`), añade las cuatro variables. Marca `RESEND_API_KEY` como secreta.
2. No cambies `RESEND_API_URL`: sólo se usa en pruebas.
3. **Cadena de proxies:** debe ser Coolify (Traefik) → Caddy (`proxy`) → Go, todo en la red Docker privada. Así la IP del visitante sale de `X-Forwarded-For` sin que el visitante pueda falsearla.
   - Si Caddy se expusiera directamente a Internet sin Traefik, seguiría siendo seguro: con un salto público, Caddy reemplaza la cabecera.
4. Redespliega el servicio `api`. En el log de arranque:
   - activado: no aparece `contact form disabled`;
   - si falta algo: aparece `contact form disabled` con los motivos (nunca la clave).
5. Comprueba que `GET https://<tu-dominio>/api/v1/public/contact` responde `{"available":true}`.

## 4. Prueba real (sólo cuando la autorices)
Está preparada, pero **no se ejecuta** hasta que Gustavo la autorice expresamente. Hasta entonces, es una puerta pendiente de WEB-008.
1. En `https://<tu-dominio>/es/contacto`, envía:
   - nombre: «Prueba BrambiLab»;
   - correo: una dirección tuya distinta de `CONTACT_TO`;
   - mensaje: «Prueba de activación del formulario, <fecha y hora>».
2. **En el panel** `/admin/contacto`, el mensaje debe pasar a **«Aceptado por Resend»** con su id. Eso significa que Resend lo aceptó, no que se entregó.
3. **En el buzón `CONTACT_TO`:** llega el aviso con asunto `Contacto BrambiLab #xxxxxxxx`. «Responder» se dirige a la dirección del visitante (Reply-To).
4. **En Resend** (*Emails*): comprueba el estado de entrega. En v1 no hay webhooks, así que el sitio no sabe si hubo rebote ni entrega.
5. Anota el resultado en el handoff de WEB-008.

## 5. Si algo falla
| Síntoma en el panel | Causa probable | Qué hacer |
|---|---|---|
| **Fallido** con `validation_error` (403) | Dominio no verificado o `CONTACT_FROM` de otro dominio | Verifica el dominio y corrige la variable. Luego «Reintentar» (usa la misma clave si sigue dentro de 23 h). |
| **Fallido** con `missing_api_key`/`restricted_api_key` | Clave ausente, revocada o sin permiso | Corrige la clave en Coolify, redespliega y reintenta. |
| **Pendiente de reintento** | 429, 5xx o red | Se reintenta solo: 30 s × 2ⁿ, con tope de 10 min y 5 intentos. |
| **Resultado incierto** (`unknown`) | Timeout, caída o 5xx fuera de la ventana de 23 h | Busca el mensaje en Resend por hora y asunto. Si no llegó, reintenta confirmando el posible duplicado: usa una clave nueva y queda auditado. |

- **Payload congelado:** cada mensaje se envía con el remitente y el destinatario vigentes cuando se recibió. Si cambias las variables, sólo afectan a los mensajes nuevos.
- **Desactivar** (`CONTACT_ENABLED=false`) detiene la recepción y los envíos. Lo pendiente queda en el panel y sigue al reactivar.

## 6. Privacidad y retención
- La página de contacto informa de la finalidad, el almacenamiento temporal y el proveedor de envío.
- Nombre, correo y mensaje se guardan en la base 30 días y después se borran junto con el trabajo, los intentos y la clave de idempotencia.
- La purga **no** borra los correos del buzón `CONTACT_TO`, de Resend ni de los backups. Documenta su retención antes del lanzamiento (WEB-008).
- La IP no se guarda: sólo una clave HMAC temporal para las cuotas, que se purga a las 24 h.
