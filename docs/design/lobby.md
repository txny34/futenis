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
| Límites caminables | `|X| <= LobbyHalfX - PlayerRadius`, `|Z| <= LobbyHalfZ - PlayerRadius`; default `LobbyHalfX = 16`, `LobbyHalfZ = 18` |
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
| 74 | `LobbyHalfZ` | m, `[8, 25]` | `18` | Medio ancho del área caminable |

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
  del pad y **abre el overlay de ese modo** (no arranca solo): 1v1/2v2 → modal **"Elegí un modo"** con la card casual de
  ese modo preseleccionada (§"Elegí un modo"); Entrenamiento → panel de entrenamiento ("Empezar").
  Mientras está abierto, controles deshabilitados y puntero liberado; Esc, ✕ o "Volver" lo cierran y devuelven el control
  en el lobby. Al confirmar se llama a `startMatch(opts)` (`configure(mode,...)`, `applyTraining` si playground,
  `reset()`), con VS intro como hoy. El sim solo re-emite `EvPadStart` tras salir y volver a entrar al pad
  (`LobbyState.Started` se limpia al cambiar de pad), así que cancelar parado sobre el pad no re-abre la pantalla.
- **JUGAR** (barra inferior) abre el modal "Elegí un modo" sin selección.
  No hace falta re-instanciar el wasm: `configure` + `reset` ya reconstruyen `Match` con la Config nueva.
- **Volver al lobby**: al cerrar la pantalla VICTORIA/DERROTA (botón "Volver al lobby"), y desde la pausa (Esc) con un
  botón **Volver al lobby**, el cliente llama `configure(3, …)` + `reset()`. El jugador reaparece en el spawn.
  En Entrenamiento no hay fin: solo Esc → Volver al lobby.
- **Sin dificultad en la UI.** El nivel del bot es automático según el ELO del invitado en esa cola
  (`lobby/bot-level.ts`, función pura): `elo < 900 → easy`, `< 1100 → normal`, `< 1300 → hard`, si no `pro`.
  La clave vieja `futenis.difficulty` se ignora. La config de entrenamiento sigue en `localStorage` (try/catch).

## 7. Cliente (UI del lobby)

- **Sin título/menú inicial**: `main.ts` arranca en el lobby (se eliminaron `titleScreen`/`mainMenu`). El menú se
  conserva como overlay: el **menú del lobby** (Esc) con Entrenamiento y Ajustes, y el modal "Elegí un modo" (JUGAR / pads 1v1-2v2).
- **Escena**: piso grande del lobby (`2·LobbyHalfX × 2·LobbyHalfZ`) con borde neón; la cancha de pádel NO se dibuja en
  `ModeLobby` (se oculta/muestra el grupo de la cancha). Misma cámara (seguimiento detrás del jugador) e input.
- **Pads**: disco emisivo de radio 1.5 m + **cartel 3D** (plano con textura de canvas generada una vez al cargar,
  billboard) con el nombre: "Entrenamiento · Lanzapelotas", "1v1 contra el bot", "2v2 contra bots".
  **Anillo de llenado** alrededor del pad que muestra `SnapLobbyHold` (0→1); pulso al `EvPadEnter`, flash al
  `EvPadStart`. Geometrías/materiales creados una vez; por frame solo se actualizan uniforms/escalas (cero allocations).
- **Menú del lobby (Esc)**: modal estilo juego (`ui/lobby-menu.ts`, mismo `panel`/`fx-stack` que la pausa). Contiene
  **Entrenamiento** (panel existente `training-view.ts`; "Volver" regresa al menú),
  **Ajustes** ("Volver" regresa al menú) y **Volver** (cierra). Los ítems son una lista simple en `main.ts`
  (`setLobbyMenu`): agregar uno es sumar `{ label, run }`.
  Abrir/cerrar: con el puntero bloqueado el navegador consume el primer Esc para soltar el pointer lock, así que
  `onLockLost` en el lobby (sin otro menú abierto) abre el menú — igual que la pausa en partido; con el puntero libre,
  `keydown Escape` lo alterna (debounce de 150 ms por si el mismo Esc llega por ambos caminos). Mientras está abierto,
  controles deshabilitados y mouse libre; al cerrar vuelve el control. En partido, Esc sigue abriendo la pausa.
- **Barra inferior**: solo el dock `[Equipo · Estilos · Misiones]  JUGAR  [Regalo · Tienda · Mercancía]`. JUGAR abre
  "Elegí un modo" (ya no hay `futenis.lastMode`).
- **HUD del lobby**: oculto el marcador/racha; la línea de ayuda abajo a la izquierda muestra "Esc menú". Sin hint de pads.
- Colores **solo** en `client/src/env/theme.ts` (tokens nuevos `lobbyFloor`, `lobbyEdge`, `padTraining`, `pad1v1`,
  `pad2v2`, `padFill`). Archivos TS < 200 líneas, strict, sin `any`.

### 7.1 Pase visual (decor, solo cliente)

`client/src/lobby/decor/*` (agregado por `LobbyDecor`, hijo del grupo de `LobbyView` → `setLobby(false)` oculta todo,
luces incluidas). Piso de canvas (paneles, juntas que brillan, logo "FUTENIS"), 3 arcos neón con color cíclico en
z=-13.5, paredes con paneles + trims instanciados, pantalla gigante (canvas redibujado a 12 Hz, marcador de la cancha
de exhibición + ticker), pósters, pads con plinto, borde animado, haz de luz y partículas, polvo flotante, conos de
luz que barren, confeti al pisar/arrancar, NPCs decorativos (mismo rig) y una **cancha de exhibición** en z=-25
(fuera de los límites) con dos atletas en un rally infinito. Nada de esto toca el sim; geometría de pads/límites igual.
Colores en `THEME.lobbyDecor`. Update sin allocations (uniforms, matrices, strings precomputados).

### 7.2 Zona 2 · Salón de la fama (`client/src/lobby/zone2/*`, solo cliente)

Detrás del spawn (extremo +Z). Para hacer lugar, el default `LobbyHalfZ` pasó de 12 a 18 (solo valor default, sin cambio
de layout) y la pared del fondo de z=+14 a z=+23. Tres pantallas enmarcadas mirando a -Z en z≈22.4: izquierda
"MEJOR ELO 1v1" (x=-8, giro 0.35 rad hacia el centro), centro "TOP VICTORIAS" (x=0), derecha "MEJOR ELO 2v2" (x=+8),
título "SALÓN DE LA FAMA" arriba. Datos **solo locales** (`profile/store.ts`): fila propia resaltada "TÚ · <nombre>"
con su stat real + zona "RANKING ONLINE — PRÓXIMAMENTE" con slots vacíos (sin nombres inventados). Podios 1-2-3 en
z=19.6 (fuera de lo caminable) bajo las pantallas laterales; el #1 muestra una estatua del rig del jugador si tiene
≥ 1 victoria, si no "¿Serás vos?". Flecha en el piso en z=10.5 "ZONA 2 · SALÓN DE LA FAMA". Canvases redibujados
solo en `onChange` del perfil; una SpotLight cálida fija dentro de `LobbyDecor.lights` (set de luces constante).
Colores en `THEME.lobbyDecor.hall`. Emisivos bajos, sin parpadeo.

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
  anillos), `client/src/lobby/bottom-bar.ts` (dock + JUGAR), `client/src/ui/lobby-menu.ts` (menú Esc: dificultad, Entrenamiento, Ajustes).
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
3. A 1.5 s sobre cada pad arranca el modo correcto (1v1/2v2 vía "Elegí un modo"; playground usa la config de
   entrenamiento guardada).
4. JUGAR abre "Elegí un modo" sin pisar pads.
5. Al terminar un partido, "Volver al lobby" reaparece en el spawn; Esc en partida/entrenamiento ofrece "Volver al lobby".
6. `assertLayout` pasa con el wasm nuevo; `?dev=1` muestra los 3 sliders del lobby.
7. No hay tienda, monedas, login ni elementos placeholder.

## Fuera de alcance (Fase 1)

Otros jugadores visibles en el lobby, chat, cuentas/login, cosméticos, tienda, emotes, NPCs, pelota libre para patear en
el lobby, colisión con decorado. Todo eso es Fase 2+.

## Perfil local y misiones

Todo es **local** (invitado, `localStorage`), sin servidor y sin jugadores falsos. El sim no cambia: el cliente cuenta
a partir de los eventos y resultados que ya existen.

### Perfil (`client/src/profile/store.ts`, API congelada)

`profileStore(): ProfileStore` (singleton) con `get()` y `onChange(cb) -> unsubscribe`. `ProfileStats`: `name`
("Invitado"), `level/xp/xpToNext`, `elo` por cola (`1v1`, `2v2`, arranca 1000), `wins/losses/streak/bestStreak`,
`perfects`, `trainings`, `bestTrainingStreak`. Clave `futenis.profile.v1`, validada al cargar (campo inválido → default,
JSON roto o storage bloqueado → perfil por defecto en memoria). La escritura (`profileWriter().update`) es interna a
`profile/`; el resto del cliente (incl. la Zona 2) solo lee.

### Cómo se alimenta (`profile/recorder.ts`)

| Fuente | Qué registra |
|---|---|
| `flow.startMatch` → `recorder().begin(cola \| "training", nivelBot, ranked)` | Abre la actividad actual |
| `GameLoop.onGameEvent` (todo evento no-lobby) | `EvKick` del slot 0 con `quality === Quality.Perfect` → `perfects++` (+5 XP); en entrenamiento `EvReturnOK`/`EvReturnFail` llevan la racha |
| `GameLoop.onMatchOver` → `flow.showResult` → `recorder().matchOver(won, games0, games1)` | Victoria/derrota, racha, XP; ELO solo en Clasificación; agrega una fila al historial |
| `flow.enterLobby` / nuevo `startMatch` → `recorder().end()` | Cierra el entrenamiento: cuenta como completado con ≥ 5 devoluciones válidas (+30 XP, `bestTrainingStreak`) |

Abandonar un partido (Esc → Volver al lobby) no cuenta como derrota ni gana XP.

- **Casual vs Clasificación** (`MatchOptions.ranked`): casual cuenta victorias/derrotas/racha/XP y deja el ELO igual;
  clasificación además mueve el ELO y multiplica la XP del partido ×1.5. Las misiones cuentan ambos.
- **XP**: victoria 100, derrota 40 (×1.5 en Clasificación), perfecto 5, entrenamiento 30. `xpToNext = 100 + 40·nivel` (sube varias veces si alcanza).
- **ELO** vs bots (solo partidos; el entrenamiento no toca ELO): solo Clasificación; rating del bot por el nivel auto-elegido — Fácil 800, Normal 1000,
  Difícil 1200, Pro 1400 — `K = 32`, `E = 1 / (1 + 10^((bot − yo)/400))`, por cola.

### Misiones (`profile/missions.ts`, clave `futenis.missions.v1`)

4 diarias (reset a medianoche local) y 3 semanales (reset el lunes local). Cada bucket guarda su clave de período; si no
coincide con la fecha actual se descarta. Métricas: `win`, `match`, `perfect`, `training` (suman) y `trainStreak` (máximo).
Reclamar suma la XP de la misión al perfil.

| Diarias | Meta | XP | Semanales | Meta | XP |
|---|---|---|---|---|---|
| Ganá 1 partido | 1 | 60 | Ganá 5 partidos | 5 | 250 |
| 10 golpes perfectos | 10 | 50 | Jugá 10 partidos | 10 | 200 |
| Completá 2 entrenamientos | 2 | 50 | 50 golpes perfectos | 50 | 200 |
| Racha de 5 en el lanzapelotas | 5 | 40 | | | |

### UI del lobby

- **Barra inferior**: dock principal (Entrenamiento y Ajustes están en el menú Esc; JUGAR abre "Elegí un modo")
  `[Equipo · Estilos · Misiones]  JUGAR  [Regalo · Tienda · Mercancía]`. JUGAR es verde (tokens `--fx-go`,
  `--fx-go-deep`, `--fx-go-ink` en `menu.css`), sin animación. Íconos SVG en línea (`lobby/icons.ts`, `currentColor`).
  Misiones pliega/despliega el panel y muestra un badge con la cantidad reclamable; los otros cinco muestran un aviso
  "Próximamente" (sin contenido falso).
- **Panel de misiones** (`lobby/missions-panel.ts`): izquierda, plegable, pestañas Día/Sem., filas con progreso, recompensa
  y botón de reclamar.
- **Tarjeta de perfil** (`lobby/profile-card.ts`): arriba a la derecha, avatar con nivel, nombre, barra "x / y XP", chips ELO.
- Todo visible solo en el lobby; el DOM se toca únicamente en `onChange` del perfil/misiones o en clicks.

### "Elegí un modo" (`ui/play-modal/*`)

Modal a pantalla completa (estilo propio, tokens `--fx-*`, sin flashes) que abren JUGAR y los pads 1v1/2v2.
- **Centro**: "Partidas casuales" (`1v1 · Casual`, `2v2 · Casual`) y "Clasificatorias" (`1v1 · Clasificación`,
  `2v2 · Clasificación`, subtítulo "Suma ELO · 1.5x XP"). Arte procedural (`art.ts`: SVG en línea de cancha, vidrio, red,
  jugadores sin paleta y la pelota; gradientes CSS). Selección única (click marca); el botón grande abajo a la derecha
  dice "ELEGÍ UN MODO" (deshabilitado) hasta elegir y luego "JUGAR", que arranca `lobbyOptions(modo, ranked)`.
- **Izquierda**: "Tu perfil" (nombre, nivel, G/P, ELO 1v1 y 2v2), leído en cada apertura.
- **Derecha**: **Historial** (últimos 20 partidos reales: fecha, modo, casual/clasificación, resultado, juegos, ΔELO;
  `localStorage` `futenis.history.v1`, `profile/history.ts`, lo escribe `recorder.matchOver`), **Información** (reglas
  cortas + controles) y **Recompensas** (aviso "Próximamente"). Sin datos falsos (ni jugadores activos, ni regiones, ni grupos).
- ✕ arriba a la derecha y Esc cierran (Esc cierra primero una sub-hoja abierta; listener en captura para que no abra el
  menú Esc). Mientras está abierto: controles deshabilitados y mouse libre; al cerrar se restauran (mismo patrón que los
  demás overlays del lobby, `main.ts lobbyOverlay`).
