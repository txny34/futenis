# Online spike (v0)

Objetivo: validar el netcode antes de invertir en cuentas/cosméticos. Dos navegadores juegan 1v1 en una sala
privada por código contra un servidor autoritativo (`cmd/server`) que corre el MISMO `sim` nativo a 120 Hz.

## 1. Arquitectura

- `game/`: Go puro. Sesión (modo, config, bots) + formato de frame (snapshots prev/curr + eventos, LE float64).
  Lo usan `wasm/` (local) y `cmd/server` (online). Dependencias: `wasm -> game`, `cmd/server -> game`, `game -> (ai, sim)`.
- `cmd/server`: WebSocket (`github.com/coder/websocket`), `-addr :8091`, `-origins localhost:8090`.
- Cliente: `online.html` + `client/src/online.ts`, `net/remote-engine.ts` implementa `StepEngine` (misma interfaz
  que el `Engine` wasm), así `GameLoop`, render y HUD no cambian.

## 2. Sala

- URL `online.html?room=CODE` (si falta, el cliente genera un código de 5 letras y lo pone en la URL para compartir).
- Primer conectado = host (slot 0), segundo = invitado (slot 1). Un tercero se rechaza (cierre 4001 "room full").
- Arranca cuando están los dos: 1v1, scoring de pádel (`game.Session.Configure(Mode1v1, Normal, 5, 0b11)`).
- Si uno se va: el otro recibe `PEER_LEFT` y la sala se cierra.

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

El servidor corre `Session.Run(inputs, 2, out)` cada 1/60 s (2 ticks por frame) con el último input de cada jugador.

## 5. Latencia simulada

`?lag=150` agrega 75 ms de ida y 75 ms de vuelta en el cliente (colas con timestamp, sin allocations por frame).
El HUD muestra sala, estado y ping (RTT medido con PING/PONG cada 1 s; incluye el lag simulado).

## 6. Fuera del spike (siguiente paso)

Predicción local del propio jugador + reconciliación, interpolación por buffer de jitter, compensación de lag en
el buffer de golpe, red por eventos de toque en vez de frames completos, 2v2, reconexión, matchmaking, cuentas.
