# mobile — ROVER Control (Android, Flutter)

Fase 1 de la app Android del rover ([issue #58](https://github.com/Codelab-ai-dev/brambiLab/issues/58)):
**conexión Wi-Fi y vídeo en vivo** del ESP32-CAM. Usa la API HTTP que ya sirve
el firmware [`rover_esp32cam`](../firmware/README.md), sin cambiarlo. El panel
web de la placa sigue siendo la referencia y no depende de esta app.

Fuera de esta fase: control de motores, Bluetooth/BLE, Arduino UNO Q, IA y
telemetría avanzada.

## Qué hace

1. **Conectar**: escribes la IP (`192.168.0.108`) o el nombre (`rover.local`)
   y la app consulta `GET http://<host>/status` (puerto 80). Solo si responde
   con el JSON del rover guarda el host en el teléfono y abre el vídeo. Un
   equipo distinto en esa IP (router, otra web) se rechaza. Un nombre se
   resuelve **una vez** al conectar y la sesión usa esa IP para `/status` y el
   vídeo: en Android la resolución de `.local` es intermitente.
2. **Vídeo**: abre `GET http://<host>:81/stream` (MJPEG) y muestra los
   fotogramas con su tasa (fps) y el RSSI, que se relee de `/status` cada 5 s.
3. **Orientación**: botones para **girar 90°** y **voltear en espejo**, por si
   la cámara va de lado o invertida en el soporte. Se guarda en el teléfono.
4. **Estados**: *Conectando*, *En vivo*, *Sin conexión* (con la causa y botón
   **Reconectar**) y *En pausa*.
5. **Pérdida de enlace**: la primera imagen tiene 10 s para llegar; con el
   vídeo en marcha, 4 s sin fotogramas (caída del Wi-Fi, rover apagado) cierran
   el stream. La app consulta `/status` para explicar la causa y ofrece
   reconectar sin reiniciar la app.
6. **Ciclo de vida**: el stream solo está abierto con la pantalla de vídeo
   visible. Se cierra al volver atrás o al pasar la app a segundo plano
   (`hidden`/`paused`) y se reabre al volver (`resumed`). Bajar la persiana de
   notificaciones (`inactive`) no lo corta.

### Por qué importa cerrar el stream

`esp_http_server` atiende **una petición a la vez por servidor** y el handler
del stream no retorna mientras el cliente siga conectado (ver
[firmware/README.md](../firmware/README.md)). Un stream abierto en la app deja
sin vídeo al panel web, y viceversa. Si la app no recibe vídeo pero `/status`
responde, lo dice: cierra el panel web u otro visor y pulsa *Reconectar*.

## Estructura

| Ruta | Contenido |
|---|---|
| `lib/main.dart` | Arranque y tema |
| `lib/src/rover_address.dart` | Validación del host y URL de `/status` y `/stream` |
| `lib/src/rover_client.dart` | Cliente de `GET /status` y errores para la interfaz |
| `lib/src/mjpeg.dart` | Decodificador `multipart/x-mixed-replace` y conexión del stream |
| `lib/src/host_store.dart` | Último host válido (`shared_preferences`) |
| `lib/src/connect_screen.dart` | Pantalla de conexión |
| `lib/src/video_screen.dart` | Vídeo, estados y ciclo de vida |
| `test/` | Pruebas unitarias y de widgets |

Dependencias: `http` y `shared_preferences`. El MJPEG se decodifica en la
propia app (sin paquete de terceros): corta por `Content-Length`, como lo envía
`stream_handler()`, y si falta busca el fin de imagen JPEG.

## Requisitos y comandos

Compilado con Flutter 3.47.5 (Dart 3.13.4), plataforma Android 37 con
build-tools 36.0.0 y el JDK de Android Studio (OpenJDK 25); el código Java y
Kotlin apunta a Java 17. `targetSdk` 36 y `minSdk` 24 (Android 7.0), los que
fija Flutter por defecto.

```sh
cd projects/001-ai-rover/mobile
flutter pub get
flutter analyze
flutter test
flutter build apk --debug          # build/app/outputs/flutter-apk/app-debug.apk
flutter run                        # con el teléfono conectado por USB o Wi-Fi
```

El build `release` (`flutter build apk --release`) firma de momento con la
clave de depuración: sirve para instalarlo a mano, no para publicarlo.

## Permisos y HTTP sin TLS

| Ajuste | Dónde | Por qué |
|---|---|---|
| `android.permission.INTERNET` | `android/app/src/main/AndroidManifest.xml` | Abrir sockets hacia el rover. La plantilla de Flutter solo lo declaraba en debug. Es un permiso normal: no se pide al usuario. |
| `android:usesCleartextTraffic="true"` | mismo archivo | El firmware solo sirve HTTP, sin TLS. |

No hacen falta permisos de ubicación, Wi-Fi ni red local: la app no escanea
redes ni descubre dispositivos, solo conecta al host que escribe el usuario.

**HTTP sin cifrar, aceptado solo en una LAN de confianza.** Cualquiera en la
misma red puede ver el vídeo y, cuando exista control, enviar órdenes. Es el
mismo nivel que ya tiene el panel web de la placa. Android no permite acotar la
excepción a rangos de IP privadas (`network_security_config` solo admite
dominios), por eso es global; la app solo construye URL hacia el host
configurado. Antes de usar el rover en redes compartidas habría que añadir
autenticación o TLS en el firmware, en otro issue.

## Probar en un teléfono Android real

Con el rover encendido y el firmware actual, teléfono y rover en la **misma
red Wi-Fi**. Apunta modelo y versión de Android, IP o nombre usado y el commit.

| # | Prueba | Cómo | Esperado (criterio del issue) |
|---|---|---|---|
| 1 | Conexión | Escribir la IP del rover y *Conectar* | Abre el vídeo; al reabrir la app el host sigue guardado (AC 1) |
| 2 | Nombre mDNS | Repetir con `rover.local` | Funciona o explica que no resuelve y sugiere la IP |
| 3 | Dirección errónea | IP de otro equipo o inexistente | Error claro, sin bloquear la pantalla |
| 4 | Vídeo | Mirar la imagen 1 min | Vídeo fluido, fps visibles, sin abrir el panel web (AC 2) |
| 5 | Wi-Fi caído | Apagar el Wi-Fi del teléfono con el vídeo abierto | En ~4 s *Sin conexión*; al reactivarlo, *Reconectar* recupera el vídeo (AC 3) |
| 6 | Rover apagado | Cortar la alimentación del ESP32-CAM | Igual que la 5; tras encenderlo, *Reconectar* funciona (AC 3) |
| 7 | Segundo plano | Botón de inicio con vídeo abierto; abrir `http://<ip>/` en el navegador del ordenador | El panel web recibe vídeo: la app soltó el stream (AC 4) |
| 8 | Volver | Volver a la app | *Conectando* y luego *En vivo* sin tocar nada |
| 9 | Salir del vídeo | Flecha atrás y abrir el panel web | El panel recibe vídeo (AC 4) |
| 10 | Cliente ocupado | Con el panel web mostrando vídeo, conectar desde la app | Mensaje de un solo stream a la vez; tras cerrar el panel, *Reconectar* funciona |
| 11 | Panel web intacto | Usar el panel web tras las pruebas | Vídeo y conducción como antes (AC 5) |

Registra el resultado en [validation.md](../validation.md). Hasta entonces
estas pruebas cuentan como **pendientes**: las pruebas automáticas usan
servidores simulados, no el ESP32-CAM.

## Limitaciones conocidas

- Probada en un Nothing A015 con Android 16 ([validation.md](../validation.md)):
  conexión por IP y vídeo medidos el 2026-10-23; pérdida de enlace, segundo
  plano y panel web pasan según informe de Gustavo del 2026-10-27, sin
  capturas.
- `rover.local` no resolvió en ese teléfono. Depende de la versión de Android y
  del fabricante; la IP siempre funciona y el firmware la imprime por serie. Si
  el router cambia la IP del rover, hay que volver a conectar.
- El teléfono tiene que estar en la red de **2,4 GHz** del rover: el ESP32 no
  usa 5 GHz y algunos routers no comunican las dos bandas.
- La app necesita buena señal en el rover: con la antena suelta (−80 dBm) cada
  fotograma de 11 kB tardaba segundos y no había vídeo.
- Los umbrales de 10 s (primera imagen) y 4 s (corte) son estimaciones;
  `firstFrameTimeout` y `stallTimeout` en `VideoScreen`.
- La orientación se corrige solo en la app: el panel web sigue mostrando la
  imagen girada. Para arreglar ambos habría que tocar `set_vflip`/`set_hmirror`
  en el firmware, fuera del alcance de #58.
- Tras una caída brusca del Wi-Fi, el ESP32 puede tardar unos segundos en
  notar que el cliente anterior se fue (envío bloqueado hasta el timeout del
  socket). Durante ese tiempo una reconexión puede fallar y hay que repetirla.
- Cada fotograma se decodifica como imagen nueva; con VGA es suficiente, pero
  no se ha medido consumo de batería ni de CPU.
