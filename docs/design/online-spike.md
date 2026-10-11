# Online spike (v0)

Objetivo: validar el netcode antes de invertir en cuentas/cosméticos. Dos navegadores juegan 1v1 en una sala
privada por código contra un servidor autoritativo (`cmd/server`) que corre el MISMO `sim` nativo a 120 Hz.

## 1. Arquitectura

- `game/`: Go puro. Sesión (modo, config, bots) + formato de frame (snapshots prev/curr + eventos, LE float64).
  Lo usan `wasm/` (local) y `cmd/server` (online). Dependencias: `wasm -> game`, `cmd/server -> game`, `game -> (ai, sim)`.
- `cmd/server`: WebSocket (`github.com/coder/websocket`), `-addr 127.0.0.1:8091` (solo local por defecto), `-origins localhost:8090`.
- Cliente: `online.html` + `client/src/online.ts`, `net/remote-engine.ts` implementa `StepEngine` (misma interfaz
  que el `Engine` wasm), así `GameLoop`, render y HUD no cambian.

## 2. Sala

- URL `online.html?room=CODE` (si falta, el cliente genera un código de 5 letras y lo pone en la URL para compartir).
- Primer conectado = host (slot 0), segundo = invitado (slot 1). Un tercero se rechaza (cierre 4001 "room full").
- Arranca cuando están los dos: 1v1, scoring de pádel (`game.Session.Configure(Mode1v1, Normal, 5, 0b11)`).
- Si uno se va (cierre 1000): el otro recibe `PEER_LEFT` y la sala se cierra.
- Si se le corta la conexión con el partido en juego, el asiento queda reservado 20 s (`rejoinGrace`, hasta 3 veces por
  asiento): el rival recibe `PEER_AWAY`, 1v1 y clasificatoria se pausan (en 2v2 casual juega el bot) y el cliente
  reintenta `/ws?rejoin=TOKEN` con el token de `SEAT`. Al volver recibe `WELCOME` + `SEAT` y el rival `PEER_BACK`.
  Si no vuelve a tiempo es un abandono normal (en clasificatoria pierde). Token desconocido o vencido: cierre 4007.
  Si el server todavía no notó el corte, la conexión nueva reemplaza a la vieja (cierre 4008, sin pausa ni aviso al rival).
  Tope de 4 reservas/reemplazos por sala. En clasificatoria, si el último conectado también se cae con otro asiento
  reservado, el partido termina sin resultado; irse a propósito (1000/1001) siempre es derrota.

## 3. Vista del invitado

El cliente siempre se dibuja como slot 0 / equipo 0 (X negativo). El servidor rota el frame del invitado 180° en Y
(X y Z negados), intercambia slot 0<->1 (2<->3) y equipo 0<->1 (`game.RotateOut1v1`), y rota su input de vuelta
(`game.RotateInput`). Una rotación no invierte la quiralidad: el spin no cambia. Solo 1v1.

## 4. Protocolo (binario LE, header de 8 bytes: [type u8][version u8 = 1][6 bytes reservados])

| Dir | Tipo | Nombre | Cuerpo |
|---|---|---|---|
| C→S | 0x01 | INPUT | 6 × f64: moveX, moveZ, targetX, targetZ, buttons, spin (vista del cliente) |
| C→S | 0x02 | PING | f64 clientMs |
| S→C | 0x81 | WELCOME | byte 2 = slot; cuerpo f64 snapLen, f64 outLen (el cliente valida contra sus constantes) |
| S→C | 0x82 | WAITING | — (solo en la sala) |
| S→C | 0x83 | FRAME | `game.OutLen` × f64: el mismo buffer de salida que el wasm (prev, curr, count, eventos), en la vista del receptor |
| S→C | 0x84 | PONG | f64 clientMs (eco) |
| S→C | 0x85 | PEER_LEFT | — |
| S→C | 0x86 | RESULT | f64 ELO antes, f64 después (clasificatoria) |
| S→C | 0x87 | SEAT | token de reconexión (32 caracteres hex ASCII), después de WELCOME |
| S→C | 0x88 | PEER_AWAY | f64 segundos que tiene el rival para volver |
| S→C | 0x89 | PEER_BACK | — |

El servidor corre `Session.Run(inputs, 2, out)` cada 1/60 s (2 ticks por frame) con el último input de cada jugador.

## 5. Latencia simulada

`?lag=150` agrega 75 ms de ida y 75 ms de vuelta en el cliente (colas con timestamp, sin allocations por frame).
El HUD muestra sala, estado y ping (RTT medido con PING/PONG cada 1 s; incluye el lag simulado).

## 6. Fuera del spike (siguiente paso)

Predicción local del propio jugador + reconciliación, interpolación por buffer de jitter, compensación de lag en
el buffer de golpe, red por eventos de toque en vez de frames completos, 2v2, reconexión, matchmaking, cuentas.

## 7. Antes de exponerlo públicamente (revisión de seguridad del spike)

Hecho (`cmd/server/limits.go` + `cmd/server/internal/netlimit`): conexiones globales (`2 × maxRooms`) y 4 por IP (HTTP 429), máximo de salas,
cierre 4003 por inactividad (20 s sin INPUT/PING con el partido en curso: una pestaña oculta deja de mandar, así que también es el AFK; el host esperando no cuenta),
4004 por rate limit (> 242 mensajes por ventana fija de 1 s), 4005 si el host sigue solo a los 5 min. La IP es la del socket TCP: detrás
de un proxy hay que confiar en `X-Forwarded-For`, decisión que va con el hosting.

Pendiente:

- Rate de accept por IP / backoff (hoy el cupo se toma antes del handshake). Detrás de un proxy, sin
  `X-Forwarded-For` confiable, el límite por IP colapsa a 4 conexiones en total.
- Rechazos de capacidad (429, sala llena, server lleno) llegan al navegador como 1006 "desconectado".

- Límites: conexiones globales y por IP, cantidad máxima de salas, timeout del host esperando rival, deadline de
  lectura por inactividad y rate limit de INPUT/PING por conexión.
- TLS: `wss://` detrás de un proxy (el cliente ya usa `wss` cuando la página es https).
- Códigos de sala: hoy 5 caracteres aleatorios (`crypto.getRandomValues`), sin secreto: el primero que llega con el
  código entra. Para público: códigos generados por el servidor o un token de invitación.
- Envíos por peer en goroutines con buffer para que un cliente lento no demore al otro (hoy acotado por el
  write timeout de 100 ms).
