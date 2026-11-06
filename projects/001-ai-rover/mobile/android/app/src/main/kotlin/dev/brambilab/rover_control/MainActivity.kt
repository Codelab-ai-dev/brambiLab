package dev.brambilab.rover_control

import android.Manifest
import android.annotation.SuppressLint
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothSocket
import android.content.pm.PackageManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodChannel
import java.io.IOException
import java.util.UUID
import java.util.concurrent.Executors

/**
 * Puerto serie Bluetooth (RFCOMM/SPP) hacia la pasarela del UNO Q (#73).
 *
 * Solo transporte: líneas de texto en los dos sentidos. El protocolo y la
 * seguridad están en Dart (`teleop_controller.dart`) y en la pasarela.
 *
 * - `brambilab/bt` (métodos): `bonded` → [{name, address}], `connect(address)`,
 *   `send(line)`, `close`.
 * - `brambilab/bt/lines` (eventos): cada línea recibida; termina al cerrarse.
 */
class MainActivity : FlutterActivity() {
    private val main = Handler(Looper.getMainLooper())
    private val io = Executors.newSingleThreadExecutor()
    @Volatile private var socket: BluetoothSocket? = null

    /** Cada conexión tiene la suya: los eventos de una anterior se descartan. */
    @Volatile private var generation = 0
    private var sink: EventChannel.EventSink? = null
    private var pendingPermission: (() -> Unit)? = null
    private var pendingDenied: (() -> Unit)? = null

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        val messenger = flutterEngine.dartExecutor.binaryMessenger
        MethodChannel(messenger, "brambilab/bt").setMethodCallHandler { call, result ->
            when (call.method) {
                "bonded" -> withPermission({ result.success(bonded()) }) {
                    result.error("permission", "Sin permiso de Bluetooth", null)
                }
                "connect" -> {
                    val address = call.argument<String>("address")
                    if (address == null) {
                        result.error("args", "Falta la dirección", null)
                    } else {
                        withPermission({ connect(address, result) }) {
                            result.error("permission", "Sin permiso de Bluetooth", null)
                        }
                    }
                }
                "send" -> {
                    val line = call.argument<String>("line") ?: ""
                    io.execute { write(line) }
                    result.success(null)
                }
                "close" -> {
                    closeSocket()
                    result.success(null)
                }
                else -> result.notImplemented()
            }
        }
        EventChannel(messenger, "brambilab/bt/lines").setStreamHandler(
            object : EventChannel.StreamHandler {
                override fun onListen(arguments: Any?, events: EventChannel.EventSink) {
                    sink = events
                }

                override fun onCancel(arguments: Any?) {
                    sink = null
                }
            },
        )
    }

    private fun adapter() =
        (getSystemService(BLUETOOTH_SERVICE) as BluetoothManager).adapter

    @SuppressLint("MissingPermission")
    private fun bonded(): List<Map<String, String>> =
        adapter()?.bondedDevices?.map {
            mapOf("name" to (it.name ?: it.address), "address" to it.address)
        } ?: emptyList()

    @SuppressLint("MissingPermission")
    private fun connect(address: String, result: MethodChannel.Result) {
        closeSocket()
        val gen = ++generation
        Thread {
            try {
                val adapter = adapter() ?: throw IOException("Este teléfono no tiene Bluetooth")
                if (!adapter.isEnabled) throw IOException("El Bluetooth está apagado")
                val device = adapter.getRemoteDevice(address)
                val s = device.createRfcommSocketToServiceRecord(SPP_UUID)
                // Sin cancelDiscovery(): la app nunca busca dispositivos y, desde
                // Android 12, esa llamada exige además el permiso BLUETOOTH_SCAN.
                s.connect()
                socket = s
                main.post { result.success(null) }
                read(s, gen)
            } catch (e: Exception) {
                main.post { result.error("connect", e.message ?: "No se pudo conectar", null) }
            }
        }.start()
    }

    /** Hilo lector: entrega líneas hasta que se cierre el socket. */
    private fun read(s: BluetoothSocket, gen: Int) {
        try {
            val reader = s.inputStream.bufferedReader(Charsets.UTF_8)
            while (true) {
                val line = reader.readLine() ?: break
                main.post { if (gen == generation) sink?.success(line) }
            }
        } catch (_: IOException) {
        } finally {
            if (socket === s) socket = null
            try {
                s.close()
            } catch (_: IOException) {
            }
            main.post { if (gen == generation) sink?.endOfStream() }
        }
    }

    private fun write(line: String) {
        val s = socket ?: return
        try {
            s.outputStream.write((line + "\n").toByteArray(Charsets.UTF_8))
            s.outputStream.flush()
        } catch (_: IOException) {
            closeSocket()
        }
    }

    private fun closeSocket() {
        val s = socket ?: return
        socket = null
        try {
            s.close() // el hilo lector termina y cierra el flujo de eventos
        } catch (_: IOException) {
        }
    }

    private fun withPermission(granted: () -> Unit, denied: () -> Unit) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S ||
            checkSelfPermission(Manifest.permission.BLUETOOTH_CONNECT) ==
            PackageManager.PERMISSION_GRANTED
        ) {
            granted()
            return
        }
        pendingPermission = granted
        pendingDenied = denied
        requestPermissions(arrayOf(Manifest.permission.BLUETOOTH_CONNECT), PERMISSION_REQUEST)
    }

    override fun onRequestPermissionsResult(
        requestCode: Int,
        permissions: Array<out String>,
        grantResults: IntArray,
    ) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode != PERMISSION_REQUEST) return
        val ok = grantResults.isNotEmpty() && grantResults[0] == PackageManager.PERMISSION_GRANTED
        val action = if (ok) pendingPermission else pendingDenied
        pendingPermission = null
        pendingDenied = null
        action?.invoke()
    }

    override fun onDestroy() {
        closeSocket()
        io.shutdown()
        super.onDestroy()
    }

    companion object {
        private val SPP_UUID: UUID = UUID.fromString("00001101-0000-1000-8000-00805f9b34fb")
        private const val PERMISSION_REQUEST = 7301
    }
}
