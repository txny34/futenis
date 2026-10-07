# Lobby mundo abierto (invitado) — Fase 1 — contrato congelado

Al abrir la web el jugador aparece **directo en un lobby 3D** (estilo hub de Roblox): camina libre por una arena neón
con **pads de partida** en el piso. Sin pantalla de título ni menú previo, solo invitado (sin login, sin cuentas, sin
persistencia más allá del `localStorage` que ya existe). Pararse sobre un pad durante `LobbyHoldTime` arranca ese modo.
Al terminar el partido/entrenamiento (o Esc → **Volver al lobby**) se vuelve al lobby.

El lobby es un **modo nuevo del sim** (`ModeLobby`): el movimiento, el salto, el dash, la stamina y la detección de pads
son física/reglas y viven en `sim/` (deterministas, cero allocations por tick). El cliente solo dibuja y reacciona a
eventos. Todo se **agrega al final** de los layouts de §20 (contact-v2.md). Nada existente cambia de índice ni de valor.

Valores actuales de referencia (leídos del código en `feature/open-lobby`, 2026-10-07):
`ModePlayground = 2`, último escalar `cfgFeedValidBounce = 71`, `CfgScalarLen = 72`, `CfgLen = 149`
(77 floats de Parts), último snapshot `SnapMachineZ = 90`, `SnapLen = 91`, último evento `EvReturnFail = 23`.

## 1. Modo

```go
const (
	Mode1v1        Mode = iota // 0
	Mode2v2                    // 1
	ModePlayground             // 2
	ModeLobby                  // 3  <- nuevo, apendeado
)
```

- `numPlayersForMode(ModeLobby) = 1` (solo el humano, slot 0, equipo 0). Sin bot, sin pelota, sin puntuación.
- `Config.Sanitize()`: `Mode` se clampa a `0..3`.
- `wasm/session.go configure`: el rango válido pasa a `0..3` (mensaje de error `mode %d not in 0..3`).

## 2. Geometría del lobby (metros, mismo sistema: Y arriba, X largo, Z ancho)

El lobby es un rectángulo **plano, sin red, sin paredes de juego** centrado en el origen. Las paredes de pádel y la red
NO existen en este modo para el sim (el cliente dibuja su propio decorado, ver §6).

| Elemento | Valor |
|---|---|
| Límites caminables | `|X| <= LobbyHalfX - PlayerRadius`, `|Z| <= LobbyHalfZ - PlayerRadius`; default `LobbyHalfX = 16`, `LobbyHalfZ = 12` |
| Spawn | `(0, 0, 6)`, mirando a -Z (hacia los pads) |
| Pad 0 · "Entrenamiento · Lanzapelotas" | centro `(-6, 0, -4)` |
| Pad 1 · "1v1 contra el bot" | centro `(0, 0, -6)` |
| Pad 2 · "2v2 contra bots" | centro `(6, 0, -4)` |
| Forma del pad | círculo de radio `LobbyPadRadius = 1.5` (constante `sim`, no Config) |

- Los centros son **constantes** en `sim/lobby.go` (`LobbyPads [LobbyPadCount]Vec2`, `LobbyPadCount = 3`), exportadas para
  tests; el cliente los replica en `client/src/lobby/pads.ts` (verificados por un test de contrato del lado Go que
  imprime los valores y por `assertLayout` vía `futenis.layout().lobbyPads` — ver §5).
- "Sobre el pad" = distancia horizontal del centro del jugador al centro del pad `<= LobbyPadRadius` **y** `OnGround`
  (saltar encima no cuenta; aterrizar sí).
- Los pads no se solapan (distancia mínima entre centros 6.3 m > 2·1.5), así que a lo sumo un pad está activo.

## 3. Config (append en `sim/config_layout.go`, después de `cfgFeedValidBounce = 71`)

| Índice | Campo | Unidad / rango | Default | Significado |
|---|---|---|---|---|
| 72 | `LobbyHoldTime` | s, `[0.3, 5]` | `1.5` | Tiempo parado sobre un pad para arrancar el modo |
| 73 | `LobbyHalfX` | m, `[10, 30]` | `16` | Medio largo del área caminable |
| 74 | `LobbyHalfZ` | m, `[8, 25]` | `12` | Medio ancho del área caminable |

- `CfgScalarLen = 75`, `CfgPartsStart = 75`, `CfgLen = 75 + 77 = 152`.
- `Sanitize()` deriva `lobbyHoldTicks = ticksOf(LobbyHoldTime)` (campo no exportado, como los demás `*Ticks`).
- Los demás campos (velocidad, salto, dash, stamina, gravedad) se reusan tal cual: el movimiento en el lobby se siente
  igual que en la cancha.

## 4. Sim (`sim/lobby.go`, archivo nuevo; `types.go` ya está en 380 líneas, no crece más que el enum y el campo)

Estado nuevo (append al final de `State`): `Lobby LobbyState`.

```go
type LobbyState struct {
	Pad      int8   // pad bajo el jugador: -1 ninguno, 0..2
	Hold     uint32 // ticks acumulados sobre Pad
	Started  bool   // ya se emitió EvPadStart; el sim queda "congelado" en el pad hasta reset/configure
}
```

- `setupLobby(s)`: `Phase = PhaseRally` (no hay saque), jugador en el spawn con `Stamina = 1`, `Ball = Ball{}` y
  **la pelota nunca se integra** (en `Match.Step`, `stepBall`/`applyBodyCollisions`/`applyStrikes`/`applyRules` se
  saltean si `Mode == ModeLobby`; se llama `stepLobby` en su lugar). `Lobby = LobbyState{Pad: -1}`.
- Movimiento: se reusa `movePlayers` completo (Sprint, Jump, Dash, stamina). Solo cambia el clamp:
  `clampPlayerToCourt` usa `clampPlayerToLobby` (rango simétrico `±(LobbyHalfX - PlayerRadius)` en X y
  `±(LobbyHalfZ - PlayerRadius)` en Z, sin `sideXRange`) cuando `Mode == ModeLobby`. Strike/Lob se ignoran
  (sin pelota no hay golpe ni `EvWhiff`).
- `stepLobby(s, ev)` cada tick, después de mover:
  1. `pad := padUnder(&s.Players[0])` (iteración en orden fijo 0..2, sin maps).
  2. Si `pad != Lobby.Pad`: si el anterior era `>= 0` emitir `EvPadLeave`; si el nuevo es `>= 0` emitir
     `EvPadEnter`; `Lobby.Pad = pad`, `Lobby.Hold = 0`.
  3. Si `pad >= 0 && !Started`: `Hold++`; si `Hold >= lobbyHoldTicks` → `Started = true`, emitir `EvPadStart`.
  4. Si `Started`, el sim no vuelve a emitir `EvPadStart` (el cliente cambia de modo; si no lo hace, salir del pad
     resetea `Started = false` vía el paso 2).
- Eventos nuevos (append al final de `EventKind`, después de `EvReturnFail = 23`). En los tres: `Player = 0`,
  **`Side = índice del pad (0..2)`** (reuso documentado del campo; en `ModeLobby` no hay equipos rivales),
  `Pos = centro del pad (Y = 0)`:

  | Valor | Evento | Cuándo |
  |---|---|---|
  | 24 | `EvPadEnter` | el jugador entró (en el piso) al pad `Side` |
  | 25 | `EvPadLeave` | salió del pad `Side` (o saltó) antes de completar |
  | 26 | `EvPadStart` | completó `LobbyHoldTime` sobre el pad `Side`: arrancar ese modo |

- Determinismo: sin `time.Now`, sin `math/rand`, sin maps, sin goroutines. Expresiones float multi-operación con
  `float64(...)`. Cero allocations por tick (`testing.AllocsPerRun` = 0 en `ModeLobby`).

## 5. Snapshot (append al final, antes de `SnapLen`)

| Índice | Campo | Valor |
|---|---|---|
| 91 | `SnapLobbyPad` | pad activo: `-1` ninguno, `0..2` |
| 92 | `SnapLobbyHold` | progreso `0..1` = `Hold / lobbyHoldTicks` (clamp a 1); `0` fuera de `ModeLobby` |

- `SnapLen = 93`. Posición/velocidad/stamina/dash del jugador ya salen por los índices existentes del slot 0.
- En otros modos ambos valen `-1` / `0` (no rompen nada).
- `futenis.layout()` agrega `modeLobby: 3`, `snapLobbyPad`, `snapLobbyHold`, `evPadEnter/Leave/Start`,
  `cfgLobbyHoldTime/HalfX/HalfZ` y `lobbyPads: [x0,z0,x1,z1,x2,z2]` + `lobbyPadRadius`; el cliente lo verifica en
  `assertLayout` (el layout del input no cambia: `inputPerPlayer = 6`).

## 6. Cambio de modo (quién decide qué)

**El sim detecta; el cliente decide.** No hay "modo dentro del modo": cambiar de lobby a partida es exactamente lo que
el cliente ya hace hoy para arrancar un partido.

- Arranque: el cliente carga wasm y llama `configure(3, difficulty, pointsToWin, 1)` + `reset()` → lobby.
- `EvPadStart(Side = p)` → el cliente mapea `p` → `GameMode` (`0 → playground`, `1 → 1v1`, `2 → 2v2`), hace el flash
  del pad y **abre la pantalla de setup de ese modo** (no arranca solo): 1v1/2v2 → `matchSetup` (dificultad
  preseleccionada desde la barra, "¡A la cancha!" / "Volver"); Entrenamiento → panel de entrenamiento ("Empezar").
  Mientras está abierta, controles deshabilitados y puntero liberado; Esc o "Volver" la cierran y devuelven el control
  en el lobby. Al confirmar se llama a `startMatch(opts)` (`configure(mode,...)`, `applyTraining` si playground,
  `reset()`), con VS intro como hoy. El sim solo re-emite `EvPadStart` tras salir y volver a entrar al pad
  (`LobbyState.Started` se limpia al cambiar de pad), así que cancelar parado sobre el pad no re-abre la pantalla.
- **JUGAR** (barra inferior) arranca directo el último modo, sin pantalla de setup.
  No hace falta re-instanciar el wasm: `configure` + `reset` ya reconstruyen `Match` con la Config nueva.
- **Volver al lobby**: al cerrar la pantalla VICTORIA/DERROTA (botón "Volver al lobby"), y desde la pausa (Esc) con un
  botón **Volver al lobby**, el cliente llama `configure(3, …)` + `reset()`. El jugador reaparece en el spawn.
  En Entrenamiento no hay fin: solo Esc → Volver al lobby.
- La dificultad elegida y la config de entrenamiento siguen en `localStorage` (try/catch) como hoy.

## 7. Cliente (UI del lobby)

- **Sin título/menú inicial**: `main.ts` arranca en el lobby (se eliminaron `titleScreen`/`mainMenu`). El menú se
  conserva como overlay: Ajustes y config de Entrenamiento desde la barra inferior, y el setup de modo al pisar un pad.
- **Escena**: piso grande del lobby (`2·LobbyHalfX × 2·LobbyHalfZ`) con borde neón; la cancha de pádel NO se dibuja en
  `ModeLobby` (se oculta/muestra el grupo de la cancha). Misma cámara (seguimiento detrás del jugador) e input.
- **Pads**: disco emisivo de radio 1.5 m + **cartel 3D** (plano con textura de canvas generada una vez al cargar,
  billboard) con el nombre: "Entrenamiento · Lanzapelotas", "1v1 contra el bot", "2v2 contra bots".
  **Anillo de llenado** alrededor del pad que muestra `SnapLobbyHold` (0→1); pulso al `EvPadEnter`, flash al
  `EvPadStart`. Geometrías/materiales creados una vez; por frame solo se actualizan uniforms/escalas (cero allocations).
- **Dificultad (decisión)**: chips de dificultad (`Fácil · Normal · Difícil · Pro`) en la **barra inferior**, siempre
  visibles en el lobby; se aplican a los pads 1v1/2v2 y a JUGAR. Opción más simple: un solo control, consistente,
  sin sub-elecciones por pad. La pantalla de setup de los pads 1v1/2v2 preselecciona esa dificultad y, si se cambia
  ahí, la barra se sincroniza (mismo valor, mismo `localStorage`). Junto a ella, un botón **Entrenamiento ⚙** abre el panel de entrenamiento existente
  (`training-view.ts`) para editar la máquina antes de pisar el pad.
- **Barra inferior**: chips de dificultad · botón grande **JUGAR** (arranca el último modo jugado, `localStorage`
  `futenis.lastMode`; default `1v1`) · **Ajustes**. Nada de tienda, monedas ni placeholders.
- **HUD del lobby**: oculto el marcador/racha; hint breve "Pará sobre un pad para jugar" (desaparece al primer
  `EvPadEnter`).
- Colores **solo** en `client/src/env/theme.ts` (tokens nuevos `lobbyFloor`, `lobbyEdge`, `padTraining`, `pad1v1`,
  `pad2v2`, `padFill`). Archivos TS < 200 líneas, strict, sin `any`.

## 8. Split de archivos (dos implementadores en paralelo)

El contrato de esta doc (índices §3/§5, eventos §4, nombres en `layout()`) es la interfaz; nadie toca archivos del otro.

**Agente A — `sim/` + `wasm/` + tests Go**
- `sim/types.go` (solo: `ModeLobby` en el enum, `numPlayersForMode`, `EvPadEnter/Leave/Start`, campo `Lobby LobbyState`
  al final de `State`).
- `sim/lobby.go` (nuevo: constantes de pads, `LobbyState`, `setupLobby`, `padUnder`, `clampPlayerToLobby`, `stepLobby`).
- `sim/lobby_test.go` (nuevo), `sim/match.go` (dispatch de `ModeLobby` en `Step`/`NewMatch`), `sim/movement.go`
  (clamp según modo), `sim/config.go`, `sim/config_layout.go`, `sim/config_sanitize.go`, `sim/snapshot.go`,
  tests de layout existentes que fijen `CfgLen`/`SnapLen`.
- `wasm/session.go` (rango `0..3`; el bot no corre en lobby), `wasm/layout.go` (nuevas claves de `layout()`),
  `wasm/*_test.go`.
- `docs/design/contact-v2.md`: apéndice corto "Lobby" con los índices nuevos, estilo de los apéndices existentes.

**Agente B — `client/` solamente**
- `client/src/snapshot.ts`, `client/src/engine.ts` (índices nuevos, `EventKind`, `assertLayout`, `MODE_ID` con `lobby: 3`),
  `client/src/dev/panel.ts` (3 sliders nuevos de Config).
- `client/src/main.ts` / `client/src/loop.ts` (arranque en lobby, `EvPadStart` → `startMatch`, volver al lobby).
  `main.ts` está en 190 líneas: extraer el flujo lobby↔partida a `client/src/lobby/flow.ts`.
- Nuevos: `client/src/lobby/pads.ts` (constantes espejo), `client/src/lobby/lobby-view.ts` (piso, borde, pads, carteles,
  anillos), `client/src/lobby/bottom-bar.ts` (dificultad, JUGAR, Entrenamiento ⚙, Ajustes).
- `client/src/ui/menu.ts`, `ui/match-screens.ts` (botón "Volver al lobby"), pausa con "Volver al lobby",
  `client/src/env/theme.ts` (tokens), `client/src/render.ts`/`view/scene.ts` (mostrar/ocultar cancha), `index.html`/CSS.
- Mientras A no mergea, B puede trabajar contra este contrato; `assertLayout` fallará hasta integrar ambos (esperado).

## 9. Criterios de aceptación

**Agente A (todo con `go test ./...` verde, `gofmt`, `go vet ./...`, `GOOS=js GOARCH=wasm go vet ./wasm/`, `-race`)**
1. Layout: `ModeLobby == 3`, `cfgLobbyHoldTime == 72`, `cfgLobbyHalfZ == 74`, `CfgScalarLen == 75`, `CfgLen == 152`,
   `SnapLobbyPad == 91`, `SnapLobbyHold == 92`, `SnapLen == 93`, `EvPadEnter == 24`, `EvPadLeave == 25`,
   `EvPadStart == 26`. Ningún índice previo cambia (test de regresión de índices existentes sigue verde).
2. `WriteConfig`/`ReadConfig` round-trip de los 3 campos; `Sanitize` clampa rangos y deriva `lobbyHoldTicks`.
3. `NewMatch(ModeLobby)`: `NumPlayers == 1`, jugador en el spawn, `Lobby.Pad == -1`, pelota quieta.
4. Caminar al pad 1 y quedarse: exactamente un `EvPadEnter(Side=1)` y un `EvPadStart(Side=1)` justo en el tick
   `lobbyHoldTicks` (180 con 1.5 s a 120 Hz) después de entrar; ninguno más aunque siga parado.
5. Salir antes de completar → `EvPadLeave`, `Hold` vuelve a 0; reentrar empieza de cero. En el aire no cuenta.
6. Clamp: correr/dashear contra cada borde nunca supera `LobbyHalf* - PlayerRadius`; en 1v1/2v2/playground el clamp
   viejo no cambia (tests existentes intactos).
7. En `ModeLobby` Strike/Lob no emiten `EvKick`/`EvWhiff`; la pelota no se mueve.
8. Determinismo: dos `Match` con la misma secuencia de inputs (soak de 10 000 ticks) dan estados idénticos;
   `AllocsPerRun(Step) == 0` en lobby. Coverage `sim` ≥ 95%. `sim/lobby.go` < 400 líneas.
9. wasm: `configure(3, …)` OK, `configure(4, …)` error; `layout()` expone las claves de §5; el bot no corre en lobby.

**Agente B (TS strict, sin `any`, archivos < 200 líneas, cero allocations por frame, colores solo en `theme.ts`)**
1. Abrir `http://localhost:8090` muestra el lobby directamente con el personaje controlable (WASD, Espacio, Shift, Q),
   sin pantalla de título.
2. Los 3 pads tienen cartel legible y anillo que se llena según `SnapLobbyHold`; se vacía al salir.
3. A 1.5 s sobre cada pad arranca el modo correcto con la dificultad de la barra inferior (playground usa la config de
   entrenamiento guardada).
4. JUGAR arranca el último modo jugado (default 1v1) sin pisar pads.
5. Al terminar un partido, "Volver al lobby" reaparece en el spawn; Esc en partida/entrenamiento ofrece "Volver al lobby".
6. `assertLayout` pasa con el wasm nuevo; `?dev=1` muestra los 3 sliders del lobby.
7. No hay tienda, monedas, login ni elementos placeholder.

## Fuera de alcance (Fase 1)

Otros jugadores visibles en el lobby, chat, cuentas/login, cosméticos, tienda, emotes, NPCs, pelota libre para patear en
el lobby, colisión con decorado. Todo eso es Fase 2+.
