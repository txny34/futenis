# Entrenamiento (Playground con máquina lanzapelotas) — contrato congelado

`ModePlayground` pasa a ser **Entrenamiento**: el humano (slot 0, equipo 0) contra una **máquina lanzapelotas**
del lado del equipo 1. Sin rival, sin bot, sin puntuación de pádel. Se cuenta la **racha de devoluciones legales**.
La máquina vive en `sim/` (los lanzamientos son física: deterministas, sin `math/rand`, `sim.Hash64` sobre el estado).

Todo lo de abajo se **agrega al final** de los layouts existentes (§20). Nada existente cambia de índice.

## 1. Config (append en `sim/config_layout.go`, después del último campo actual)

Rangos: cada bola toma un valor determinista al azar dentro de `[Min, Max]` (`Hash64` del tick + contador de
lanzamientos). `Sanitize()`: clamp de cada campo y, si `Min > Max`, se intercambian.

| Campo | Unidad / rango | Default | Significado |
|---|---|---|---|
| `MachineZMin`, `MachineZMax` | m, `[-4.7, 4.7]` | `-3`, `3` | Posición lateral de la máquina (Izquierda–Derecha) |
| `MachineXMin`, `MachineXMax` | m desde la red, `[2, 9.7]` | `8`, `9.5` | Profundidad de la máquina en el campo 1 (X positivo) |
| `FeedZMin`, `FeedZMax` | m, `[-4.7, 4.7]` | `-3.5`, `3.5` | Dirección del tiro: Z del pique objetivo en el campo del humano |
| `FeedXMin`, `FeedXMax` | m desde la red, `[1, 9.7]` | `4`, `8.5` | Profundidad del tiro: \|X\| del pique objetivo en el campo 0 |
| `FeedShots` | bitmask `1..7` | `1` | 1 = Derecha (drive), 2 = Globo (lob), 4 = Remate (tiro descendente fuerte). Cada bola elige uno de los bits activos. 0 → 1 |
| `FeedPowerMin`, `FeedPowerMax` | `[0, 1]` | `0.3`, `0.7` | Potencia (interpola la velocidad del tipo de golpe) |
| `FeedQualityMin`, `FeedQualityMax` | `[0, 1]` | `0.7`, `1` | Calidad: 1 = va exacto al objetivo; menor = más desvío (radio hasta ~1.5 m) |
| `FeedOnDemand` | 0/1 | `0` | 0 = Continua (lanza sola tras `FeedInterval`); 1 = A demanda (espera `Match.Feed()`) |
| `FeedInterval` | s, `[0.5, 5]` | `1.5` | Tiempo entre que la bola anterior "muere" y el próximo lanzamiento |
| `FeedVolleyOnly` | 0/1 | `0` | 1 = Solo volea: hay que golpear antes de que pique en tu campo |
| `FeedValidBounce` | `1` o `2` | `1` | Piques permitidos en tu campo antes de golpear (1 = reglamento de pádel, 2 = más fácil). Ignorado si `FeedVolleyOnly` |

## 2. Sim

- `setupPlayground`: `NumPlayers = 1` (solo el humano). Slot 1 queda inactivo: no se mueve, no golpea, no se dibuja.
- Estado nuevo en `State` (agregar al final de la struct): `Feed FeedState` con `Count uint32` (lanzamientos),
  `NextTick uint32` (tick del próximo lanzamiento, 0 = esperando demanda), `Live bool`, `Bounces uint8`
  (piques en el campo 0 de la bola actual), `Struck bool` (el humano ya la golpeó), `Streak uint16`, `Best uint16`.
- **Lanzamiento**: la bola aparece en la boca de la máquina (`MachineX/Z` muestreados, Y = 1.0 m) y se lanza con el
  solver real (`solveLaunch`) hacia el objetivo muestreado. Drive y remate pasan la red siempre (reusar
  `ensureNetClearance`); el remate sale desde Y = 2.6 m con trayectoria descendente; el globo usa `ShotLob`.
- **Juicio de la bola** (solo `ModePlayground`, en `rules.go`; no toca la puntuación de pádel):
  - **Acierto** (Streak++ , Best = max): el humano la golpea respetando `FeedVolleyOnly`/`FeedValidBounce`, la bola
    pasa la red y su primer pique es dentro del campo 1.
  - **Fallo** (Streak = 0): pique en el campo 0 más allá de lo permitido, golpe que va a la red/afuera/pica en el campo
    0, `EvWhiff` con la bola viva... o la bola sale sin ser golpeada.
  - Tras acierto o fallo la bola "muere": `Live = false` y, si `FeedOnDemand == 0`, `NextTick = Tick + FeedInterval`.
- Eventos nuevos (append al final de `EventKind`): `EvFeed` (lanzó la máquina; `Pos` = boca), `EvReturnOK`
  (`Player` = 0), `EvReturnFail`.
- `Match.Feed()`: dispara un lanzamiento ahora si no hay bola viva (reemplaza a `RespawnBall` en Playground).
- Cero allocations por tick; determinismo; TDD; coverage `sim` ≥ 95%.
- Aclaraciones de implementación (Go): `FeedState` agrega al final `MouthX, MouthZ float64` y `Shot uint8`
  (para `SnapMachineX/Z`). `Match.Feed()` agenda el lanzamiento para el próximo `Step` (así `EvFeed` sale por la
  lista de eventos). Falla del humano también si la bola, sin tocar, queda quieta en el campo 0 o vuelve a cruzar
  la red tras picar. Falla de la máquina (su bola pega en la red o pica en su campo sin tocar el campo 0): la bola
  muere sin evento y la racha no cambia. `EvReturnFail.Player` = -1.

## 3. Snapshot (append al final, antes de `SnapLen`)

`SnapStreak`, `SnapBestStreak`, `SnapFeedCount`, `SnapFeedIn` (segundos hasta el próximo lanzamiento; `-1` =
esperando demanda o bola viva), `SnapMachineX`, `SnapMachineZ` (posición de la última boca, para dibujar la máquina).

## 4. wasm

- `respawnBall()` pasa a llamar a `Match.Feed()` (el nombre JS se mantiene; agregar alias `feed()`).
- `layout()` refleja los nuevos campos; el cliente los verifica con `assertLayout`.
- El bot NO corre en Playground.

## 5. Cliente

- Menú: "Playground" → **Entrenamiento**, abre un panel como la referencia (3 tarjetas: **Pista**, **Golpe**, **Sesión**)
  con sliders de **doble extremo** para cada rango, chips para golpes/alimentación/bote/devolución válida, minimapa de la
  cancha que muestra la zona de la máquina (rosa) y la zona de pique (cian). Botones **Volver** y **Empezar**.
  Los valores se guardan en `localStorage` (try/catch) y entran por `Config` (pasan por `Sanitize`).
- HUD: contador grande de **racha** + **mejor**, cartel en cada acierto (`¡+1!`) y al cortar la racha
  (`RACHA: N`), cuenta regresiva del próximo lanzamiento. Con `FeedOnDemand` la tecla actual de respawn = "lanzar".
- Máquina visible: modelo procedural simple (caja + cañón) en `SnapMachineX/Z`, con flash al lanzar (`EvFeed`).
- Colores solo en `theme.ts`, cero allocations por frame, archivos TS < 200 líneas.
