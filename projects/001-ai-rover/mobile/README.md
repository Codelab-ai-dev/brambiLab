# mobile — ROVER Control (Android, Flutter)

App Android del rover. Usa la API HTTP que ya sirve el firmware
[`rover_esp32cam`](../firmware/README.md), sin cambiarlo. El panel web de la
placa sigue siendo la referencia y no depende de esta app.

- Fase 1 ([#58](https://github.com/Codelab-ai-dev/brambiLab/issues/58), cerrada):
  conexión Wi-Fi y vídeo en vivo.
- Fase 2 ([#62](https://github.com/Codelab-ai-dev/brambiLab/issues/62)):
  conducción con `/drive` y `/stop`. Ver [Conducir](#conducir).

- Fase 3 ([#73](https://github.com/Codelab-ai-dev/brambiLab/issues/73)):
  **mando manual sin vídeo** por la pasarela WebSocket del Arduino UNO Q. Es
  la pantalla que abre la app ahora; ver [Mando por el UNO Q](#mando-por-el-uno-q-73).
  Las pantallas del ESP32-CAM siguen en el código (el ESP32-CAM está dañado).

Fuera de alcance: Bluetooth/BLE, IA y telemetría avanzada.

## Mando por el UNO Q (#73)

La app habla con [`ros/teleop_gateway.py`](../ros/README.md#pasarela-de-teleoperación-73),
nunca con ROS 2 directamente, por uno de dos transportes:

- **Bluetooth** (por defecto): puerto serie RFCOMM con el UNO Q vinculado
  («gus»), sin red ni ordenador; sirve con el rover en la powerbank. El puente
  nativo está en `android/.../MainActivity.kt` (sin librerías de terceros) y
  pide el permiso «Dispositivos cercanos» (`BLUETOOTH_CONNECT`).
- **Red / adb**: WebSocket `ws://<host>:8765` (ver la vía por adb abajo).

- **Conectar** con el rover vinculado (o host y puerto) y el token (el de `teleop_token` en el UNO Q). Se
  guardan en el teléfono, el token sin cifrar. Conectar **no arma**.
- **Armar** es un botón aparte. Armado, la app manda una orden cada 100 ms,
  también (0, 0) en reposo; la pasarela desarma a los 300 ms sin órdenes.
- **Cruceta** de pulsación mantenida: soltar manda (0, 0) en el acto.
  **STOP** actúa al apoyar el dedo, para y desarma.
- **Velocidad**: 25–100 % de los límites de la pasarela (0,2 m/s y 2 rad/s
  por defecto). En marcha, el giro solo corrige (factor 0,55), como el panel.
- **Estado**: «Pedido» es lo que manda la app; «Rover» es lo que confirma el
  STM32 en `/rover/state` (armado y velocidad aplicada a cada lado). La latencia
  es la del `ack` de la pasarela.
- **Seguridad**: perder la conexión, desconectar, salir de la pantalla o que la
  app deje de estar en primer plano (incluido `inactive`) manda STOP y cierra.
  Al volver hay que conectar y armar otra vez; nada se reenvía. Un aviso
  `disarmed` de la pasarela olvida los botones.
- Sin vídeo: conducir solo con el rover a la vista, en un sitio despejado.

Código: `lib/src/teleop_link.dart` (WebSocket con `dart:io` y Bluetooth por
canal de plataforma), `teleop_controller.dart`
(lógica), `teleop_screen.dart`, `teleop_store.dart`. Pruebas:
`test/teleop_controller_test.dart` y `test/teleop_screen_test.dart`.

### Sin Wi-Fi en el UNO Q: por adb

Mientras el Wi-Fi del UNO Q no funcione ([diagnóstico](../ros/README.md#wi-fi-del-uno-q)),
el teléfono llega a la pasarela a través del ordenador:

```text
teléfono 127.0.0.1:8765 ──adb reverse (Wi-Fi)──▶ ordenador :8765 ──adb forward (USB)──▶ UNO Q :8765
```

1. Teléfono y ordenador en la misma red. En el teléfono: *Opciones de
   desarrollador → Depuración inalámbrica → Vincular dispositivo con código QR*.
   El QR se genera en el ordenador con el contenido
   `WIFI:T:ADB;S:<nombre>;P:<clave>;;` (p. ej. `qrencode -o qr.png …`), y
   `adb pair <ip:puerto> <clave>` con el servicio `_adb-tls-pairing._tcp` que
   anuncia el teléfono (`adb mdns services`).
2. `adb -s <UNO Q> forward tcp:8765 tcp:8765`
3. `adb -s <teléfono> reverse tcp:8765 tcp:8765`
4. En la app: host `127.0.0.1` (el valor por defecto), puerto 8765.

`adb reverse` se pierde si se reinicia el servidor adb o el teléfono se
desconecta: repetir los pasos 2 y 3.

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
| `lib/src/video_screen.dart` | Vídeo, estados, ciclo de vida y cuándo se puede conducir |
| `lib/src/drive_mix.dart` | Mezcla diferencial, copia de `mix()` del panel web |
| `lib/src/drive_link.dart` | `/drive` y `/stop` por una sola conexión HTTP |
| `lib/src/drive_controller.dart` | Envío periódico, STOP, pérdida de control |
| `lib/src/drive_pad.dart` | Cruceta, STOP y velocidad |
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

## Conducir

Los controles están debajo del vídeo (en horizontal, a su derecha): cruceta de
avance, retroceso y giro, **STOP** en el centro y velocidad.

- **Mezcla:** la misma que el panel web (`mix()` en `index_html.h`), probada
  contra su salida en JavaScript. En parado, izquierda y derecha giran en sitio;
  avanzando, corrigen (factor 0,55). Dos dedos a la vez trazan curva.
- **Velocidad:** 80–255, como el deslizador del panel; **empieza en 120** (el
  panel, en 190).
- **Luz:** botón bajo la velocidad que recorre Apagada → Baja → Media → Alta
  (64, 128 y 255 en la escala de `/light`; el firmware limita el brillo real).
  Muestra el nivel que confirma el rover. Se apaga al salir de la pantalla de
  control. Un fallo de la luz no corta la conducción, y con un firmware sin
  `/light` la app avisa de que hay que cargar la versión nueva.
- **▲▼ invertidos:** chip bajo la velocidad, guardado en el
  teléfono. Cambia el signo del avance y deja el giro igual: un giro en sitio
  se ve igual desde cualquier lado. Hace falta si la cámara mira hacia la
  trasera eléctrica del rover, o si el cableado invirtió los motores.
- **Envío:** `/drive` al apoyar el dedo y cada **150 ms** mientras siga
  apoyado, con una sola petición en vuelo. Al soltar, `/stop` y nada más:
  la app no envía órdenes en reposo.
- **Estado:** la línea sobre la cruceta muestra lo que **confirma el rover** en
  la respuesta de cada `/drive` (`l / r` y latencia), no lo que la app cree
  haber enviado. La barra inferior toma el RSSI de esas respuestas.

### Seguridad

| Situación | Qué hace la app |
|---|---|
| Arranque | Parada y con los controles desactivados |
| Vídeo no está *En vivo* (conectando, error, pausa) | Controles desactivados y `/stop`: no se conduce a ciegas |
| Soltar, o el sistema cancela el gesto | `/stop` |
| **STOP** | Actúa al apoyar el dedo. Invalida lo que esté en vuelo, manda `/stop` y otro `/stop` de cierre cuando termina el `/drive` que viajaba |
| `/drive` falla o no responde en **400 ms** | Pérdida de control: `/stop`, aviso en rojo y **no se vuelve a mover hasta soltar y pulsar de nuevo**, aunque vuelva la red |
| Persiana de notificaciones, diálogo del sistema | `/stop`; el vídeo sigue |
| Segundo plano, salir de la pantalla | `/stop` y cierre de la conexión |
| Android mata la app | Nada que hacer desde la app: para el **watchdog del firmware a los 500 ms** |

**Orden de las órdenes.** `/drive` y `/stop` van por **una sola conexión
HTTP** (`maxConnectionsPerHost = 1`), así que llegan al ESP32 en el orden en
que salieron: un `/stop` no adelanta a un `/drive` anterior. Hay una prueba
contra un servidor local que falla si se permiten más conexiones. Un `/drive`
que ya estaba en camino puede aplicarse hasta 500 ms tras STOP; el `/stop` de
cierre lo neutraliza. Garantizarlo de forma estricta exigiría números de
secuencia en el firmware.

**Un solo controlador.** El firmware no distingue clientes: si el panel web y
la app conducen a la vez, se pisan. No conduzcas desde los dos a la vez.

### Prueba física de conducción

**Con las ruedas en el aire** (el rover sobre una caja o un soporte) mientras
no exista el interruptor de potencia ([validation.md](../validation.md)). Ten
a mano el cable de la batería para desconectarla. Panel web **cerrado**.

| # | Prueba | Cómo | Esperado (criterio de #62) |
|---|---|---|---|
| D1 | Arranque | Abrir la app y conectar | Ruedas quietas; controles desactivados hasta *En vivo* |
| D2 | Avanzar | Mantener ▲ dos segundos y soltar | Las cuatro ruedas hacia delante; paran al soltar (AC 2) |
| D3 | Direcciones | Igual con ▼, ◀ y ▶ | Atrás; giro en sitio a izquierda y derecha (AC 2) |
| D4 | Curva | ▲ y ▶ a la vez | Un lado más rápido que el otro |
| D5 | Velocidad | Repetir D2 a 80 y a 255 | A 80 giran (lento); a 255, al máximo |
| D6 | STOP | Mantener ▲ y pulsar STOP con otro dedo, sin soltar ▲ | Paran y no vuelven a arrancar aunque ▲ siga pulsado (AC 3) |
| D7 | Wi-Fi caído | Mantener ▲ y apagar el Wi-Fi del teléfono | Paran en ≤ 0,5 s (watchdog); aviso de pérdida; al volver la red no arrancan solas (AC 4) |
| D8 | Rover sin red | Mantener ▲ y alejar el router o apagarlo | Igual que D7 |
| D9 | Segundo plano | Mantener ▲ y pulsar Inicio | Paran (AC 5) |
| D10 | Persiana | Mantener ▲ y bajar la persiana de notificaciones | Paran; el vídeo sigue |
| D11 | Salir | Mantener ▲ y pulsar atrás | Paran (AC 5) |
| D12 | Panel web | Cerrar la app y conducir desde `http://<ip>/` | Funciona igual que antes (AC 7) |
| L1 | Luz | Con el firmware nuevo, recorrer los cuatro niveles | Cada nivel más brillante; *Apagada* apaga (AC 8) |
| L2 | Bandas | Luz *Alta* mirando una pared | El vídeo no muestra bandas horizontales (AC 8) |
| L3 | Consumo | Luz *Alta* y ▲ a 255 durante 10 s | Sin reinicio: `uptime_s` sigue subiendo (AC 8) |
| L4 | Temperatura | Luz *Alta* 2 minutos | El LED no quema al tacto; si quema, bajar `LIGHT_MAX_DUTY` |

Solo cuando D1–D11 pasen con las ruedas en el aire, y con el interruptor de
potencia instalado, repetir D2–D7 en el suelo. Anota en
[validation.md](../validation.md) el teléfono, el commit y qué pasó en cada
fila, incluidos los fallos.

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

Estas son las pruebas de la fase 1 (vídeo). Las de conducción están en
[Prueba física de conducción](#prueba-física-de-conducción). Registra el
resultado en [validation.md](../validation.md): las pruebas automáticas usan
servidores simulados, no el ESP32-CAM.

## Limitaciones conocidas

- Conducción **sin probar todavía en el rover**: solo pruebas automáticas
  (mezcla contra la salida del panel en JavaScript, controlador con reloj
  simulado, transporte contra un servidor HTTP local y pantalla con gestos
  simulados).
- Con poca señal Wi-Fi, una sola respuesta de `/drive` por encima de 400 ms
  cuenta como pérdida de control y obliga a soltar y volver a pulsar. Si pasa a
  menudo con buena cobertura, revisar ese umbral (`driveTimeout` en
  `HttpDriveLink`).
- Los botones se mantienen pulsados con el dedo; TalkBack no permite mantener
  pulsado, así que la conducción no es accesible con lector de pantalla.

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
