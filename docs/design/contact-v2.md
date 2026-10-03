# Contacto v2 — partes del cuerpo, timing, trayectoria analítica y spin

Estado: **aprobado (decisiones cerradas, §17)** · Alcance: `sim/`, `wasm/`, `client/`, nuevo `ai/` · Referencia de diseño: Rocket League.

> Pilar: "fácil de aprender, dificilísimo de dominar". Pocos inputs, física continua, mecánicas
> emergentes, recursos visibles (stamina = boost), transferencia de momentum. Máximo 1 toque por lado
> (no cambia).

---

## 1. Objetivos y no-objetivos

**Objetivos**

1. El jugador **apunta a un punto** de la cancha rival (cursor) y la pelota *llega* ahí si el contacto es
   perfecto. La habilidad está en el **timing**, la **posición del cuerpo** y la **carga**, no en adivinar
   un vector.
2. Contacto por **parte del cuerpo** (pie, rodilla, pecho, hombro, cabeza, chilena) elegida por la sim
   según la altura relativa de la pelota. Cada parte tiene potencia, precisión y modificadores propios.
3. **Calidad de contacto** (perfecto / bueno / forzado) → desviación **determinista** de destino y potencia.
4. **Salto**, **sprint con stamina** (modelo boost RL) y **momentum**: la velocidad del jugador se suma al tiro.
5. **Spin** (top/back, rueda del mouse, 3 niveles) que altera piques en piso y vidrio ("la bandeja muere en el vidrio").
6. Todo determinista, cero allocs por tick, FMA-safe; la trayectoria entre toques sigue siendo función pura
   del estado post-contacto → red por **eventos de toque**.
7. Bot de IA determinista fuera de `sim`, y modo de **tuning en vivo** de `Config` desde el navegador.

**No-objetivos (YAGNI)**

- Efecto Magnus en vuelo (el spin sólo actúa en los rebotes; ver §9).
- Animaciones/modelos 3D, IK. El cliente sólo recibe `Part` + `Quality` para feedback.
- Red real (sólo se define qué viaja). Rollback/predicción se implementan después.
- Pases/controles "de amortiguar" (1 toque por lado: no hay recepción + tiro).
- Flip/dodge aéreo y pelota que sale por arriba del vidrio y vuelve ("por cuatro"): **v3**. Hoy, pelota
  sobre `WallHeight` = fuera inmediato.

---

## 1.1 Frecuencias y tiempo

| Qué | Frecuencia | Nota |
|---|---|---|
| Sim (`TickRate`) | **120 Hz** (`Dt = 1/120 s`) | igual que la física de Rocket League |
| Render | sin tope (144/240 Hz) | interpolación entre estado previo y actual |
| Snapshots de red (futuro) | 60 Hz | el servidor manda 1 de cada 2 ticks; eventos de toque van siempre |

**Regla**: ningún valor de diseño se expresa en ticks fijos. `Config` guarda **duraciones en segundos** y
la sim deriva los ticks con `ticksOf(sec) = int(math.Round(sec*TickRate))` (al crear/sanitizar `Config`).
Así cambiar `TickRate` no rompe el tuning. Todas las cifras en ticks de este doc son a 120 Hz.

**Tarea #1 de la Wave 0**: `sim/types.go` `TickRate` 60 → 120. Toda constante en ticks existente pasa a
derivarse de `TickRate` (ej. `PointPauseTicks = TickRate*3/2` ya lo hace → 180) y **todos los tests
existentes deben seguir pasando** (los que hardcodean ticks se reescriben en segundos·`TickRate`).

---

## 2. Contrato de input — `PlayerInput` v2

Principio: **la sim recibe niveles (held), no flancos.** Los flancos (press/release) los detecta la sim
comparando con `Player.Held` del tick anterior. Ventajas: `step(n)` con el mismo input en N ticks es
correcto (hoy `Kick` edge-triggered se repite N veces o se pierde), el input es idempotente → apto para
reenvío en red y para el bot.

```go
type PlayerInput struct {
    MoveX, MoveZ     float64 // -1..1, dirección deseada (sim normaliza si |v|>1)
    TargetX, TargetZ float64 // metros, punto de destino en el piso de la cancha RIVAL
    Buttons          uint8   // bitmask de niveles, ver abajo
    Spin             int8    // -1 backspin (cortado) / 0 plano / +1 topspin (rueda del mouse)
}

const (
    BtnStrike uint8 = 1 << iota // click izq mantenido: cargar; al soltar: tiro tipo Drive. En saque: autopase
    BtnLob                      // click der mantenido: cargar; al soltar: tiro tipo Lob
    BtnJump                     // Space
    BtnSprint                   // Shift
)
```

**Reemplaza**: `Kick` → flanco de release de `BtnStrike|BtnLob`; `AimX/AimZ` → `TargetX/TargetZ`;
`Loft` → tipo de tiro (Drive/Lob) + solver; `Power` → carga medida **por la sim** (`ChargeTicks`, saturada en `ticksOf(ChargeMaxTime)`).
La carga deja de vivir en el cliente (`CHARGE_TIME_S` en `input.ts` desaparece).

**Sanitización** (`sanitizeInput`, sin allocs):

| Campo | Regla |
|---|---|
| MoveX/MoveZ | NaN/Inf → 0; clamp −1..1 |
| Spin | clamp a {−1, 0, +1} |
| TargetX | NaN/Inf → centro de la cancha rival (`±CourtHalfLength/2`); clamp al lado rival: `[TargetMinFromNet, CourtHalfLength−TargetWallMargin]` con signo `forwardSign(side)` |
| TargetZ | NaN/Inf → 0; clamp `±(CourtHalfWidth−TargetWallMargin)` |
| Buttons | `& btnMask` (bits desconocidos → 0) |
| Strike+Lob simultáneos | gana el que se presionó primero (latcheado en `Player.ShotType` al press) |

`TargetX` se sanitiza **con el lado del jugador**, así que `sanitizeInput(in, side)` cambia de firma.

---

## 3. Estado nuevo

```go
type Player struct {
    Pos, Vel     Vec3     // Vel.Y ahora es real (salto)
    Side         int8
    OnGround     bool
    Held         uint8    // Buttons del tick anterior (detección de flancos)
    Stamina      float64  // 0..1
    StaminaLock  uint16   // ticks restantes sin regenerar tras vaciarse
    ChargeTicks  uint16   // ticks con Strike/Lob mantenido (satura en ticksOf(ChargeMaxTime))
    ShotType     ShotType // latcheado al press: ShotDrive | ShotLob
    Buffer       uint8    // ticks restantes de intención de golpe (0 = sin intención)
    BufCharge    float64  // carga 0..1 congelada al release
    BufTarget    Vec3     // destino congelado al release (Y = BallRadius)
    BufSpin      int8     // spin congelado al release
    StruckAt     uint32   // tick del último golpe propio (gracia de colisión con el cuerpo, §7.4)
}

type Ball struct {
    Pos, Vel Vec3
    Spin     float64 // -1..1, constante en vuelo, decae en cada rebote
    Tossed   bool    // saque: la pelota ya fue lanzada (autopase)
}

type ShotType uint8 // ShotDrive=0, ShotLob=1
type BodyPart uint8 // PartNone=0, PartFoot, PartKnee, PartChest, PartShoulder, PartHead, PartBicycle
type Quality  uint8 // QualPerfect=0, QualGood, QualForced
```

`State` sigue siendo copiable por valor (rollback). Nada de punteros ni slices nuevos.

### 3.1 `Config` — parámetros nuevos (todos en `State.Cfg`, tuneables en vivo)

| Campo | Unidad | Inicial | Nota |
|---|---|---|---|
| PlayerSpeed (existe) | m/s | 6.0 | velocidad crucero |
| SprintSpeed | m/s | 8.5 | con Shift y stamina > 0 |
| PlayerAccel | m/s² | 40 | en piso; momentum real, no velocidad instantánea |
| PlayerAirAccel | m/s² | 12 | control aéreo reducido |
| PlayerGravity | m/s² | 16 | más "snappy" que la pelota |
| JumpSpeed | m/s | 6.0 | apex ≈ 1.125 m, aire ≈ 0.75 s |
| StaminaDrain | 1/s | 0.50 | 2 s de sprint continuo |
| StaminaRegen | 1/s | 0.33 | ~3 s de 0 a 1 |
| StaminaLockTime | s | 0.75 | sin regen tras vaciarse (90 ticks) |
| JumpStaminaCost | 0..1 | 0.05 | por salto; no se puede saltar con menos (frena spam de chilenas) |
| ChargeMaxTime | s | 0.9 | carga plena (108 ticks) |
| ChargeMoveFactor | 0..1 | 0.8 | multiplica la velocidad mientras carga |
| StrikeBufferTime | s | 0.1 | ventana de intención (12 ticks) |
| BodyRestitution | 0..1 | 0.3 | rebote de la pelota en el cuerpo (§7.4) |
| BodyGraceTime | s | 0.1 | el que golpeó no colisiona con su cuerpo (12 ticks) |
| ServeTossHeight | m | 1.0 | altura de la pelota en la mano antes del autopase |
| ServeTossOffset | m | 0.5 | adelante del sacador (hacia la red) |
| ServeTossSpeed | m/s | 4.5 | Vy del autopase → apex ≈ 2.03 m, vuelta a 1.0 m en ≈ 0.92 s |
| MomentumFactor | — | 0.35 | fracción de la vel. del jugador que se suma al tiro |
| MomentumMax | m/s | 4.0 | tope del aporte de momentum |
| LobApexMin / LobApexMax | m | 4.0 / 7.0 | apex del globo según carga |
| DriveNetClearance | m | 0.25 | margen sobre `NetHeight+BallRadius` del tiro ideal |
| TargetMinFromNet | m | 1.0 | |
| TargetWallMargin | m | 0.3 | |
| QualPerfectMax / QualGoodMax | — | 0.25 / 0.60 | umbrales de error normalizado `u` |
| DevPowerMax | — | 0.15 | ±15% de potencia en contacto forzado |
| SpinFloorFriction | — | 0.25 | Δ de fricción horizontal por unidad de spin |
| SpinFloorRest | — | 0.15 | Δ de restitución vertical por unidad de spin |
| SpinWallRest | — | 0.45 | backspin pleno quita 45% de restitución en vidrio |
| SpinWallDrop | m/s | 1.5 | backspin empuja hacia abajo al tocar vidrio |
| SpinDecay | — | 0.5 | spin se multiplica por esto en cada rebote |
| PointPauseTime | s | 1.5 | pausa post-punto (hoy `PointPauseTicks`, 180 ticks) |
| KickReach, KickMinSpeed, KickMaxSpeed | — | — | **se eliminan** (pasan a la tabla de partes) |

Los `*Time` se convierten a ticks con `ticksOf` en `Config.Sanitize()` y se cachean en campos derivados
no tuneables (`stLockTicks`, `chargeMaxTicks`, `bufTicks`, `graceTicks`): cero cálculo por tick.

La tabla de partes (§6) también vive en `Config` como `Parts [PartCount]PartSpec` (array fijo) para
poder tunearla en vivo.

---

## 4. Salto

```go
// stepPlayerVertical: sólo salta desde el piso, en el flanco de BtnJump, si alcanza la stamina.
if p.OnGround && pressed(p, in, BtnJump) && p.Stamina >= cfg.JumpStaminaCost {
    p.Vel.Y = cfg.JumpSpeed
    p.OnGround = false
    p.Stamina -= cfg.JumpStaminaCost   // no dispara StaminaLock salvo que llegue a 0
}
if !p.OnGround {
    p.Vel.Y -= float64(cfg.PlayerGravity * Dt)
}
p.Pos.Y += float64(p.Vel.Y * Dt)
if p.Pos.Y <= 0 { p.Pos.Y, p.Vel.Y, p.OnGround = 0, 0, true }
```

- Sin doble salto ni dodge (flip RL = **v3**).
- En el aire la aceleración horizontal usa `PlayerAirAccel`: comprometerse a un salto es una decisión.

## 5. Movimiento, sprint y stamina (modelo boost RL)

```go
target := dir * speed              // speed = SprintSpeed si sprint activo, si no PlayerSpeed
if p.ChargeTicks > 0 { target *= ChargeMoveFactor }
accel := OnGround ? PlayerAccel : PlayerAirAccel
p.Vel.XZ = approach(p.Vel.XZ, target, accel*Dt)   // mueve hacia target, |Δ| <= accel*Dt
```

Stamina por tick:

```go
sprinting := held(in, BtnSprint) && p.Stamina > 0 && moving(in)
switch {
case sprinting:
    p.Stamina = max(0, p.Stamina - StaminaDrain*Dt)
    if p.Stamina == 0 { p.StaminaLock = cfg.stLockTicks }
case p.StaminaLock > 0:
    p.StaminaLock--
default:
    p.Stamina = min(1, p.Stamina + StaminaRegen*Dt)
}
```

- Drena **sólo mientras se mantiene y hay movimiento** (Shift parado no gasta).
- Recurso visible: barra en HUD desde el snapshot.
- Stamina se resetea a 1 en `resetForServe`.

---

## 6. Partes del cuerpo

`hRel = ball.Pos.Y − player.Pos.Y` (altura de la pelota sobre los pies del jugador, incluye salto).
`dH = |ball.XZ − player.XZ|` (distancia horizontal).

| Parte | hMin | hIdeal | hMax | Alcance horiz. (m) | Vel. min–max (m/s) | Precisión (dev. base / máx, m) | Drive | Lob | Spin | Extra |
|---|---|---|---|---|---|---|---|---|---|---|
| Foot | 0.00 | 0.20 | 0.45 | 1.00 | 10 – 26 | 0.15 / 1.6 | ✔ | ✔ | ±1 | |
| Knee | 0.35 | 0.60 | 0.95 | 0.85 | 8 – 18 | 0.25 / 2.0 | ✔ | ✔ | ±0.5 | |
| Chest | 0.95 | 1.20 | 1.40 | 0.60 | 5 – 11 | 0.40 / 2.2 | ✔ | ✔ | 0 | potencia baja, muy estable |
| Shoulder | 1.30 | 1.50 | 1.65 | 0.70 | 6 – 14 | 0.35 / 2.4 | ✔ | ✘ | 0 | |
| Head | 1.60 | 1.85 | 2.10 | 0.55 | 7 – 17 | 0.20 / 1.8 | ✔ | ✔ | −0.5..0 | remate de cabeza |
| Bicycle | 1.70 | 2.10 | 2.50 | 0.80 | 16 – 30 | 0.30 / 2.8 | ✔ | ✘ | +1 | requiere `!OnGround` y `BufCharge ≥ 0.9` |

```go
type PartSpec struct {
    HMin, HIdeal, HMax, Reach   float64
    SpeedMin, SpeedMax          float64
    DevBase, DevMax             float64
    SpinMin, SpinMax            float64
    AllowLob                    bool
}
```

Solapes intencionales (knee/foot, chest/shoulder, shoulder/head, head/bicycle): la sim elige la parte
con **menor error** (§7.2); en empate gana la de menor índice (determinista).
Huecos (0.95 entre knee y chest es cerrado; >2.5 m sin parte) → no hay contacto.
El spin pedido se **clampa** a `[SpinMin, SpinMax]` de la parte: "no se corta con el pecho".

---

## 7. Resolución del golpe

### 7.1 Intención y buffer

- **Press** de Strike o Lob: `ShotType` se latchea, empieza `ChargeTicks` (satura en `chargeMaxTicks`).
  Mientras carga, la velocidad se multiplica por `ChargeMoveFactor` (0.8).
- **Release**: `Buffer = bufTicks` (12), `BufCharge = ChargeTicks/chargeMaxTicks`,
  `BufTarget`, `BufSpin` se congelan; `ChargeTicks = 0`.
- Cada tick con `Buffer > 0` se evalúa el contacto; si no hubo contacto al llegar a 0 → `EvWhiff`.
- Re-presionar durante el buffer cancela el buffer viejo (no se acumulan intenciones).

"Mejor tick de la ventana" **sin mirar el futuro real**: se compara el error de *ahora* contra el error
**predicho del tick siguiente** (pelota avanzada con `predictBall`, pura, sin eventos; jugador avanzado con
su `Vel`). Se golpea cuando el error deja de mejorar, o en el último tick del buffer.

### 7.2 Algoritmo (pseudo-Go)

```go
// applyStrikes reemplaza applyKicks. Evalúa ambos lados y después elige (§7.3). Max 1 contacto por tick.
func applyStrikes(s *State, in [2]PlayerInput, ev []Event) []Event {
    var cand [2]candidate                            // array fijo, cero allocs
    for i := int8(0); i < 2; i++ {
        p := &s.Players[i]
        updateCharge(p, in[i], &s.Cfg)               // flancos, carga, latch de buffer, autopase (§7.5)
        if p.Buffer == 0 || !canKick(s, i) { continue }
        p.Buffer--
        part, u := bestPart(&s.Cfg, p, s.Ball.Pos)   // u = error normalizado 0..1, PartNone si no alcanza
        if part == PartNone {
            if p.Buffer == 0 { ev = append(ev, whiff(s, i)) }
            continue
        }
        if p.Buffer > 0 && improvesNextTick(s, p, u) { continue } // esperar al tick óptimo
        cand[i] = candidate{ok: true, part: part, u: u}
    }
    w := pickStriker(s, &cand)                       // §7.3; -1 si nadie
    if w < 0 { return ev }
    s.Players[w].Buffer = 0
    return strike(s, w, cand[w].part, cand[w].u, ev)
}

// partError: error normalizado; >1 = fuera de alcance.
func partError(ps *PartSpec, hRel, dH float64) float64 {
    if hRel < ps.HMin || hRel > ps.HMax || dH > ps.Reach { return 2 }
    half := ps.HMax - ps.HIdeal
    if hRel < ps.HIdeal { half = ps.HIdeal - ps.HMin }
    eh := math.Abs(hRel-ps.HIdeal) / half
    er := dH / ps.Reach
    return math.Max(eh, er)                         // 0 = perfecto
}

func strike(s *State, i int8, part BodyPart, u float64, ev []Event) []Event {
    p, ps := &s.Players[i], &s.Cfg.Parts[part]
    q := qualityOf(&s.Cfg, u)
    h := strikeHash(s.Tick, i, s.Ball.Pos)
    tgt := deviateTarget(p.BufTarget, ps, u, h, i)       // §8
    speed := shotSpeed(ps, p.BufCharge, u, h, &s.Cfg)    // §8
    speed += momentumBonus(p.Vel, s.Ball.Pos, tgt, &s.Cfg)
    v := solveLaunch(&s.Cfg, s.Ball.Pos, tgt, p.ShotType, speed, p.BufCharge) // §9
    s.Ball.Vel = clampLen(v, ps.SpeedMax+s.Cfg.MomentumMax)
    s.Ball.Spin = clamp(float64(p.BufSpin), ps.SpinMin, ps.SpinMax)
    p.StruckAt = s.Tick
    if s.Phase == PhaseServe { s.Phase = PhaseRally }
    return append(ev, kickEvent(s, i, part, q))
}
```

Restricciones de `bestPart`: `Lob` sólo con partes `AllowLob` (si la mejor parte no permite lob, se usa
como Drive); `Bicycle` sólo si `!OnGround && BufCharge >= 0.9`. Cada función < 50 líneas; `bestPart`
itera un array fijo de 6 → cero allocs.

### 7.3 Golpe simultáneo (desempate determinista)

Si ambos lados tienen candidato en el mismo tick (pelota sobre la red), gana:
1. menor `u` (mejor contacto);
2. si `u` es igual bit a bit: el lado que **no** tocó último (`Rally.LastKicker`);
3. si no hubo toque (saque): el sacador.

El perdedor conserva su `Buffer` (puede golpear después si la regla lo permite; normalmente el 1-toque lo
deja sin opción). Nada depende del orden de iteración → justo para ambos lados.

### 7.4 Colisión pelota–cuerpo

Cada jugador es una **cápsula** vertical: segmento en `Pos.XZ`, `Y ∈ [Pos.Y+PlayerRadius,
Pos.Y+PlayerHeight−PlayerRadius]`, radio `PlayerRadius` (0.35 m, alto total 1.8 m). La hitbox
vive en `sim` y es **idéntica para todas las skins**.

Después de `applyStrikes` y del paso de la pelota, para cada jugador (lado 0 y 1, resultado independiente
del orden porque un tick admite a lo sumo un contacto: si hay dos, gana el de menor distancia y en empate
se aplica §7.3 paso 2–3):

```
skip si s.Tick − p.StruckAt < graceTicks           // el que acaba de golpear no se bloquea a sí mismo
c   = punto más cercano del segmento a Ball.Pos
d   = Ball.Pos − c ; dist = |d|
si dist < PlayerRadius + BallRadius y dot(Ball.Vel − p.Vel, n) < 0:   // n = d/dist (dist==0 → n = forward(side))
    vRel   = Ball.Vel − p.Vel
    vn     = dot(vRel, n)
    Ball.Vel = p.Vel + vRel − n·vn·(1 + BodyRestitution)
    Ball.Pos = c + n·(PlayerRadius + BallRadius)  // depenetrar
    Ball.Spin *= SpinDecay
    emitir EvBodyHit{Side: p.Side}
```

**Regla**: `EvBodyHit` **cuenta como toque** de ese lado (igual que `EvKick` para `judgeEvent`): si ese
lado ya tocó (`LastKicker == side`, ej. la pelota rebota en el propio cuerpo tras el golpe pasada la
gracia) → falta, punto para el rival. Un body hit tras el toque rival es un "toque" legal pero sin control
(no hay solver): la pelota rebota donde caiga. `rules.go` cambia sólo en tratar `EvBodyHit` como `EvKick`.

### 7.5 Saque (reglas reales de pádel, adaptadas a pie)

> **Anula el autopase de la versión original de este documento.** Decisión del coordinador
> (prevalece sobre el texto anterior de esta sección y sobre la fila 3 de la tabla de §17):
> el saque de pádel real, adaptado a que se golpea con el cuerpo en vez de con la mano/paleta.

- **Dos saques** por punto; doble falta = punto para el receptor (igual que pádel/tenis real).
- **Press** de Strike (en `PhaseServe`): la pelota se **suelta** a la altura de la cintura
  (`ServeTossHeight` ≈ 1.0 m) delante del sacador (`ServeTossOffset`), **sin** impulso hacia arriba
  (`ServeTossSpeed` = 0 por defecto — ya no es un autopase, es un drop). Gravedad activa desde la
  suelta, emite `EvToss`. El mismo press empieza la carga; el sacador se puede mover (a 0.8× mientras
  carga), igual que antes.
- La pelota debe **picar una sola vez** en el piso, **detrás de la línea de saque** del lado del
  sacador (`|X| > ServiceLineDist` = 6.95 m desde la red) y **antes** de que el sacador la golpee.
  Golpearla sin que haya picado, o después de un segundo pique, es falta.
- El sacador debe golpear con **pie o rodilla** (altura de contacto baja: `hRel` restringe la parte a
  `{Foot, Knee}` por la tabla de §6 — un saque no se puede cabecear ni pechar).
- El saque va **en diagonal**: debe caer primero en el **cajón de saque diagonal** del receptor (entre
  la red y la línea de saque, del lado contrario de la línea central en Z respecto de la posición del
  sacador). El sacador alterna cajón derecho/izquierdo **cada punto** (regla real de pádel/tenis):
  `serveBoxForScore(Score) = (Score[0]+Score[1]) % 2` (0 = derecha, 1 = izquierda, visto desde el
  fondo de ese sacador). Pura, en `sim`; wave1 la llama desde `resetForServe` y desde el juez de saque.
- **Falta** (`EvServeFault`): la pelota cae fuera del cajón diagonal correcto, toca la red y no cruza,
  toca una pared antes de picar, o el sacador no llega a golpearla (whiff, o la pelota pica dos veces
  antes del golpe).
  - 1ª falta de un punto → se repite el saque (`Rally.ServeAttempt` 1 → 2), sin penalidad más que eso.
  - 2ª falta consecutiva (doble falta) → punto para el receptor.
- **Let** (`EvLet`): la pelota toca la red pero **aun así** cae dentro del cajón correcto → se repite
  el mismo saque sin que cuente como falta (`Rally.ServeAttempt` no cambia).
- Una vez que el pique es válido y el sacador golpea, la pelota puede tocar vidrio de fondo/lateral
  como en el juego normal: a partir de ahí el punto se juega igual que cualquier intercambio.

**Estado nuevo (contrato wave 0; el juez de saque completo es wave 1):**

```go
type RallyState struct {
    // ... LastTeam, LastPlayer, BouncesOnSide, PauseTicks (sin cambios) ...
    ServerPlayer int8  // qué jugador (no equipo) saca este punto (serverPlayerIndex)
    ServeAttempt uint8 // 1 = primer saque, 2 = segundo saque (tras una falta)
    ServeBox     int8  // 0 = cajón derecho, 1 = cajón izquierdo (alterna por punto)
}
```

```go
// Nuevos EventKinds (append, no reordenar): EvWhiff, EvJump, EvBodyHit, EvToss,
// EvServeFault, EvLet
```

Nuevo campo de `Config`: `ServiceLineDist` (m, default 6.95) — distancia red → línea de saque, donde
el saque debe picar antes de ser golpeado. `ServeTossHeight`/`ServeTossOffset` se mantienen (altura/
adelanto de la suelta); `ServeTossSpeed` default pasa a 0 (ya no hay autopase hacia arriba, pero se
deja el campo por si se quiere tunear una asistencia pequeña).

---

## 8. Calidad y desviación determinista

```go
func qualityOf(c *Config, u float64) Quality // u<=QualPerfectMax → Perfect; u<=QualGoodMax → Good; resto Forced
```

**Hash** (splitmix64, sin `math/rand`, sin estado oculto):

```go
func strikeHash(tick uint32, side int8, b Vec3) uint64 {
    x := uint64(tick) | uint64(uint8(side))<<32
    x ^= math.Float64bits(b.X) ^ bits.RotateLeft64(math.Float64bits(b.Y), 21) ^
         bits.RotateLeft64(math.Float64bits(b.Z), 42)
    return splitmix64(x)
}
func unitPair(h uint64) (a, b float64) { // dos uniformes en [-1,1)
    a = float64(h>>11)*0x1p-52 - 1          // 53 bits
    h2 := splitmix64(h)
    b = float64(h2>>11)*0x1p-52 - 1
    return
}
```

**Desviación de destino** (sin trigonometría → sin dudas de bit-exactitud):

```
R      = DevBase + u² · (DevMax − DevBase)          // m; u² castiga mucho lo forzado
(a, b) = unitPair(h)
dir    = normalize(target − ballXZ)                 // eje longitudinal del tiro
perp   = (−dir.Z, dir.X)
target' = target + dir·(a·R) + perp·(b·R·0.6)       // más error en largo que en lateral
```

Luego se re-clampa a la cancha rival **sólo en Z** (en X puede quedar corto/largo: tiro errado real; si
cae del lado propio o fuera, son las reglas existentes las que lo castigan).

**Desviación de potencia**: `speed *= 1 + c·DevPowerMax`, `c = a·u` (perfecto ≈ 0 de error).

Todas las expresiones escritas una operación por línea o envueltas en `float64(...)`.

---

## 9. Velocidad inicial analítica (exacta al integrador discreto)

En vez de la fórmula continua con drag (exponenciales, `math.Exp` no garantizado bit-exacto entre
arquitecturas), resolvemos **la recurrencia exacta del integrador** de `stepBall`:

```
v_{n} = k · (v_{n-1} − g·Dt·ŷ),   k = 1 − AirDrag·Dt
p_{n} = p_{n-1} + v_n·Dt
```

Sea `N` el número de ticks de vuelo hasta el destino y `S = Σ_{i=1..N} kⁱ = k(1−kᴺ)/(1−k)`.

**Horizontal** (sin gravedad): `p_N − p_0 = Dt · v0 · S` →

```
v0_xz = (target_xz − ball_xz) / (Dt · S)
```

**Vertical**: `v_n = kⁿ v0 − g·Dt·k(1−kⁿ)/(1−k)`; sumando:

```
y_N − y_0 = Dt · [ v0·S − g·Dt · k/(1−k) · (N − S) ]
v0_y = ( (y_T − y_0)/Dt + g·Dt·k/(1−k)·(N − S) ) / S        // y_T = BallRadius
```

Lineal en `v0`, sin raíces. `kᴺ` se calcula con un loop de multiplicaciones (`N ≤ 600` a 120 Hz, `float64(...)`
en cada paso) — nada de `math.Pow`. Si `AirDrag == 0`: `S = N`, `k/(1−k)·(N−S)` → límite `N(N−1)/2`
(rama explícita). La pelota aterriza **exactamente** en el tick N en el target (test de oro, §14).

**Elección de N por tipo de tiro**

- **Drive**: velocidad horizontal deseada `vh = lerp(SpeedMin, SpeedMax, charge)·(1+dev) + momentum`.
  `N = ceil(d / (vh · Dt · k̄))` con `k̄ ≈ S/N` (dos iteraciones de punto fijo, determinista). Luego se
  chequea la altura en el tick de cruce de la red (`n_net` derivado de la misma fórmula horizontal); si
  `y(n_net) < NetHeight + BallRadius + DriveNetClearance`, `N++` y se re-resuelve (máx 12 iteraciones,
  acotado). El tiro **ideal** siempre pasa; el desviado puede no pasar (red = castigo legítimo).
- **Lob**: apex deseado `H = lerp(LobApexMin, LobApexMax, charge)`. Aproximación sin drag:
  `T = √(2(H−y0)/g) + √(2(H−y_T)/g)`, `N = round(T/Dt)`; luego solve exacto (con drag el apex real queda
  levemente más bajo; aceptable y tuneable).
- Tope: `|v0| ≤ SpeedMax + MomentumMax`. Si el solve pide más, se escala `v0` → la pelota **queda corta**.
  Es la mecánica: desde el fondo con la rodilla no llegás al rincón rival.

**Momentum**: `bonus = clamp(MomentumFactor · dot(p.Vel_xz, dir_tiro), −MomentumMax, MomentumMax)`.
Correr hacia el tiro = más rápido (menos N); de espaldas = más lento. Saltar no suma a Y (el solver manda).

**Spin y solver**: el spin **no afecta el vuelo** (sin Magnus) → el solver sigue siendo exacto y la red
no necesita simular aerodinámica extra. El spin actúa en los rebotes:

## 10. Modelo de spin (sólo en rebotes)

`s = Ball.Spin ∈ [−1, 1]` (+ top, − back).

**Piso** (`resolveFloor`):
```
fric = clamp(FloorFriction + SpinFloorFriction·s, 0.4, 1.05)  // top "salta hacia adelante", back frena
rest = clamp(FloorRestitution · (1 − SpinFloorRest·s), 0, 0.95) // top pica más bajo
Ball.Spin *= SpinDecay
```
**Vidrio/pared** (`reflectWalls`, componente normal):
```
restW = WallRestitution · (1 + SpinWallRest·min(s,0))           // backspin pleno: 0.7 → 0.385
if s < 0 { Vel.Y −= SpinWallDrop · (−s) }                        // "la bandeja muere en el vidrio"
Ball.Spin *= SpinDecay
```

**Control del spin**: rueda del mouse, 3 niveles discretos −1 / 0 / +1 (el cliente guarda el nivel y lo
manda como nivel en `PlayerInput.Spin`); indicador en el retículo. En gamepad: bumpers.

---

## 11. Eventos, snapshot y layout wasm

### 11.1 Eventos

```go
type Event struct {
    Kind    EventKind
    Tick    uint32
    Pos     Vec3
    Side    int8
    Part    BodyPart // sólo EvKick
    Quality Quality  // sólo EvKick
    Vel     Vec3     // EvKick: velocidad post-contacto (red + feedback)
    Spin    float64  // EvKick
}
// Nuevos kinds (append, no reordenar): EvWhiff, EvJump, EvBodyHit, EvToss
```

`judgeEvent` trata `EvBodyHit` igual que `EvKick` (toque del `Side`). **La regla de 1 toque por lado queda
intacta**; `EvWhiff`/`EvJump`/`EvToss` son ignorados por las reglas.

**Fixes base (ya en curso, se mantienen)**: eventos edge-triggered.
- Pelota en reposo sobre el piso emite **un solo** `EvBounceFloor` (al entrar en contacto), no uno por tick.
- `EvOut` se emite **una vez** al salir de los límites, no en cada tick que sigue afuera.

### 11.2 Snapshot (se **agrega al final**, índices viejos no cambian)

```
SnapBallSpin
SnapBallTossed
SnapP0VelX, SnapP0VelZ, SnapP0Stamina, SnapP0Charge, SnapP0OnGround, SnapP0Buffer, SnapP0Flags
SnapP1VelX, ... (mismo bloque para P1)
SnapLen
```

`SnapP0Y`/`SnapP1Y` ya existen y pasan a tener valor real. `Charge` = `ChargeTicks/chargeMaxTicks`.
`Flags` = `ShotType | (StaminaLock>0)<<1`.

### 11.3 wasm

- `inputPerPlayer`: 7 → 6 `[moveX, moveZ, targetX, targetZ, buttons, spin]`.
- `eventStride`: 6 → 13 `[kind, tick, x, y, z, side, part, quality, vx, vy, vz, spin, _pad]` (o 12; fijar).
- Nuevo `setConfig(f64[ConfigLen])` / `getConfig()` (§13). Nuevo `setMode(vsAI)` (§12).
- `layout_test.go` cubre los nuevos offsets.

### 11.4 Cliente

- `input.ts`: mouse → raycast al plano Y=0 → `TargetX/Z` (en coordenadas de mundo); click izq/der/Space/
  Shift → `buttons` como **niveles** (se elimina `clearKicks` y el cálculo de carga); rueda → nivel de spin.
- `engine.ts`: loop con acumulador a `Dt = 1/120`, render sin tope interpolando (alpha = acumulador/Dt).
- `snapshot.ts`/`engine.ts`: nuevos índices y strides.
- `hud.ts`: barra de stamina, anillo de carga, indicador de spin, flash de calidad (perfecto/bueno/forzado)
  y nombre de parte en `EvKick`/`EvBodyHit`. Retículo en el target. El círculo de desviación `R` se dibuja
  **sólo en tutorial y `?dev=1`**; en juego normal el dominio es "de tacto".
- `render.ts`: jugador con Y real; sombra en el piso para leer saltos; en dev, cápsula de hitbox.

---

## 12. Bot de IA

Paquete **`ai/`** a nivel raíz (no dentro de `sim`): `ai → sim`, `sim` no conoce a `ai`.

```go
package ai
type Bot struct { Level Level; delay [maxDelay]sim.PlayerInput; head uint8 } // ring fijo, cero allocs
func (b *Bot) Think(s *sim.State, side int8) sim.PlayerInput
```

- Determinista: sólo lee `State`; "errores humanos" vía el mismo `splitmix64` exportado por `sim`
  (`sim.Hash64`) con semilla `tick`.
- `sim` exporta `PredictBall(b Ball, cfg *Config, ticks int) Ball` (pura, sin eventos) — el bot y
  `improvesNextTick` la comparten.
- Estrategia v1: predecir la pelota, buscar el primer tick en que entra en la franja de una parte
  (preferencia Foot/Head), moverse a ese punto (sprint si no llega), soltar el click `bufTicks/2` (6 ticks)
  antes, target = rincón lejos del rival ± ruido por nivel.
- `Level`: `ReactionTime` (s), `AimNoise`, `ChargeSkill`, `UsesSpin`, `UsesJump`.
- Corre dentro del wasm (lado 1 en modo vsAI) y, en el futuro, en el servidor (mismo código Go).

## 13. Tuning en vivo

- `sim/config_layout.go`: `CfgGravity = iota ... CfgLen` (incluye la tabla de partes aplanada) +
  `WriteConfig(c *Config, dst []float64)` / `ReadConfig(src []float64) Config` + `Config.Sanitize()`
  (clamps físicos: restituciones 0..1, velocidades > 0, `HMin<=HIdeal<=HMax`).
- wasm: `getConfig()` / `setConfig()` aplicado **entre frames** (nunca en medio de un `step`).
- Cliente: panel dev propio (sin deps) con `?dev=1`: sliders por campo, export/import JSON, "reset defaults".
- Online: el panel se deshabilita; la `Config` la fija el servidor y viaja en el handshake.

## 14. Implicancias de red

Un **evento de toque** debe contener lo necesario para reproducir la trayectoria sin re-simular el contacto:

```
TouchEvent { Tick uint32; Side int8; Kind (Kick|BodyHit|Toss); Part; Quality; BallPos Vec3; BallVel Vec3; Spin float64 }
```

- Con `Config` idéntica y el integrador determinista, `(Pos, Vel, Spin)` en `Tick` ⇒ trayectoria exacta
  hasta el próximo toque (rebotes incluidos). Los float64 viajan como **bits** (8 bytes LE), nunca texto.
- Snapshots de estado a 60 Hz (1 de cada 2 ticks de sim); los `TouchEvent` se mandan siempre, con su
  tick exacto de 120 Hz.
- El servidor es quien resuelve el contacto (hash, calidad, solver). El cliente puede **predecir** su
  propio golpe y corregir con el `TouchEvent`.
- Inputs: `PlayerInput` completo por tick de sim (120 Hz; se pueden agrupar de a 2 por paquete); como son **niveles**, son
  idempotentes y tolerantes a reenvío.
- El estado de jugador nuevo (Stamina, Charge, Buffer) es parte del `State` → entra en rollback sin cambios.

---

## 15. Plan de tests (TDD, `go test ./sim/... ./ai/...`, coverage ≥ 80%)

**Input**
1. `TestSanitize_TargetClampedToRivalHalf` (lado 0 y 1, NaN → centro rival).
2. `TestSanitize_UnknownButtonBitsDropped`.
3. `TestEdges_HeldStrikeAcrossMultiStepIsSinglePress`.

**Movimiento/salto/stamina**
4. `TestAccel_ReachesSpeedInExpectedTicks` (6 m/s / 40 m/s² = 0.15 s = 18 ticks).
5. `TestJump_OnlyFromGround_ApexMatchesFormula` (`v²/2g` ± 1 tick de discretización).
6. `TestJump_AirAccelReduced` + `TestJump_CostsStamina_BlockedWhenInsufficient`.
7. `TestStamina_DrainsOnlyWhileSprintingAndMoving`.
8. `TestStamina_DepletionTriggersLockThenRegen` (`ticksOf(0.75)` = 90 ticks sin regen).
9. `TestStamina_ResetOnServe`.
10. `TestCharge_SaturatesAtMax_SlowsMovement` (108 ticks, 0.8×).
10b. `TestTicksOf_DerivedFromTickRate` (duraciones → ticks, round).

**Partes/calidad**
11. Tabla: `TestBestPart_ByHeight` (cada fila de §6 en su hIdeal → esa parte, u == 0).
12. `TestBestPart_OverlapPicksLowerError_TieLowerIndex`.
13. `TestBicycle_RequiresAirborneAndFullCharge`.
14. `TestLobDowngradesToDriveOnShoulder`.
15. `TestSpinClampedPerPart` (pecho → 0).
16. `TestQualityThresholds`.

**Buffer/timing**
17. `TestBuffer_StrikesWithinWindow_WhenBallArrivesLate` (release 50 ms = 6 ticks antes → contacto).
18. `TestBuffer_ExpiresEmitsWhiff`.
19. `TestBuffer_WaitsForBetterTick` (error decreciente → golpea en el mínimo).
20. `TestBuffer_RepressCancelsOld`.

**Determinismo/desviación**
21. `TestStrikeHash_Stable` (vectores de oro hardcodeados: mismo resultado en amd64, arm64 y wasm vía CI).
22. `TestDeviation_ZeroAtPerfectCenter_BoundedByR`.
22b. `TestSimultaneousStrike_LowerErrorWins_ThenNonLastToucher`.
23. `TestMatch_ReplayIdentical` (1000 ticks con inputs grabados, dos corridas → State igual bit a bit).

**Solver**
24. `TestSolveLaunch_LandsExactlyOnTargetAtTickN` (simular N ticks con `PredictBall`, |err| < 1e-9),
    tabla con drag 0 / 0.1 / 0.3.
25. `TestSolveLaunch_DriveClearsNet` (desde el fondo, target corto).
26. `TestSolveLaunch_LobApexWithinTolerance`.
27. `TestSolveLaunch_OverSpeedClampedFallsShort`.
28. `TestMomentum_RunningIntoShotIncreasesSpeed_Bounded`.

**Spin**
29. `TestFloorBounce_TopspinKeepsMoreHorizontal_LowerBounce`.
30. `TestWall_BackspinDiesOnGlass` (restitución y caída de Vy).
31. `TestSpin_DecaysPerBounce`.

**Cuerpo/saque**
31b. `TestBodyHit_BouncesOffCapsule_Restitution` + `TestBodyHit_GraceSkipsStriker`.
31c. `TestBodyHit_CountsAsTouch_DoubleTouchIsFault`.
31d. `TestServe_TossThenStrikeOnlyDescending` + `TestServe_TossLandsUnstruck_Replays`.

**Reglas/eventos/snapshot**
32. `TestRules_OneTouchPerSideUnchanged` (suite existente verde; `rules.go` sólo suma `EvBodyHit`).
32b. `TestEvents_FloorRestSingleBounce` + `TestEvents_OutEmittedOnce` (edge-triggered).
33. `TestKickEvent_CarriesPartQualityVelSpin`.
34. `TestSnapshot_NewFields` + `TestSnapshot_OldIndicesStable`.
35. `TestStep_ZeroAllocs` (`testing.AllocsPerRun == 0` sobre `Step` con strikes activos).
36. `TestConfigLayout_RoundTrip` + `TestConfigSanitize`.

**AI**
37. `TestBot_Deterministic` (mismo State → mismo input). 38. `TestBot_ReturnsServe`.
39. `TestBot_ZeroAllocs`. 40. `TestBotVsBot_RallyLastsNTouches` (smoke, semilla fija).

**wasm**: `layout_test.go` para strides y decode de `buttons`.

---

## 16. Tareas de implementación (por ownership, contrato fijo)

**Wave 0 — contrato (1 agente, bloqueante, ~1 h)**
- **T0.1 (tarea #1)** `TickRate` 60 → 120 en `sim/types.go`; constantes en ticks derivadas de `TickRate`;
  `go test ./sim/...` verde **antes** de seguir.
- T0 `sim/types.go`, `sim/config_layout.go` (constantes), `sim/snapshot.go` (índices): tipos, enums,
  `PlayerInput` v2, `Player`/`Ball`/`Event` v2, campos de `Config` + `DefaultConfig`, índices de snapshot,
  `CfgLen`. Stubs que compilen (`panic("todo")` prohibido en Step: stubs no-op). Tests existentes adaptados
  al input nuevo.

**Wave 1 — paralelo (archivos disjuntos)**
- T1 sim `player.go`: aceleración, salto (con costo), stamina, carga/flancos, `ticksOf` (tests 3–10b).
- T2 sim `trajectory.go`: `solveLaunch`, `PredictBall`, momentum (24–28).
- T3 sim `quality.go`: hash, calidad, desviación (16, 21, 22).
- T4 sim `spin.go` + hooks en `physics.go` (29–31).
- T5 wasm `layout.go`/`main.go`: strides, `setConfig/getConfig`, `setMode` (layout tests).
- T6 client: `input.ts` (mouse raycast, niveles), `engine.ts`, `snapshot.ts` contra los índices de T0.
- T7 client: `hud.ts` (stamina, carga, spin, calidad) + panel dev `devpanel.ts`.

**Wave 2**
- T8 sim `strike.go` + `bodyparts.go`: buffer, `bestPart`, `applyStrikes`, desempate, autopase
  (11–15, 17–20, 22b, 31d, 33) — usa T2/T3.
- T8b sim `body.go` + `rules.go`: colisión cápsula, `EvBodyHit` como toque (31b, 31c).
- T9 `ai/`: bot (37–40) — usa `PredictBall` (T2) y contrato T0.

**Wave 3**
- T10 integración: `Step` cableado, zero-alloc (35), replay (23), wasm vsAI, `go vet`, coverage.

Regla para agentes: **nadie modifica tipos/índices de T0**; si un contrato no alcanza, se frena y se
escala.

---

## 17. Decisiones (cerradas)

| # | Tema | Decisión |
|---|---|---|
| 1 | Frecuencias | Sim 120 Hz; render sin tope con interpolación; snapshots de red 60 Hz (§1.1). |
| 2 | Spin | Rueda del mouse, 3 niveles: back / plano / top (§10). |
| 3 | Saque | **Override (coordinador, post-aprobación):** reglas reales de pádel adaptadas a pie — drop (sin impulso), pique obligatorio tras la línea de saque, cajón diagonal, 2 saques, let/falta (§7.5). Reemplaza el autopase original de esta fila. |
| 4 | Salto | Cuesta `JumpStaminaCost = 0.05` (§4). |
| 5 | Carga | Ralentiza a 0.8× (§7.1). |
| 6 | Círculo de desviación | Sólo tutorial / dev; asunto del cliente (§11.4). |
| 7 | Cuerpo | La pelota rebota en la cápsula del jugador; `EvBodyHit` cuenta como toque (§7.4). |
| 8 | Flip/dodge aéreo | v3. |
| 9 | Pelota sobre el vidrio | v3; hoy fuera inmediato. |
| 10 | Golpe simultáneo | Menor error `u`, luego el que no tocó último, luego el sacador (§7.3). |
| 11 | Eventos base | `EvBounceFloor` en reposo y `EvOut` edge-triggered (§11.1). |

---

## 18. Dirección de arte (nota)

Indie con identidad propia, no realista. **Un único rig humanoide compartido** por todos los personajes y
skins (las animaciones se hacen una vez). La **hitbox vive en `sim`** (cápsula de §7.4 + tabla de partes
§6) y es idéntica para todas las skins: lo visual **nunca** afecta la física ni las reglas.

---

## 19. Modos y 2v2

> Decisión del coordinador (contrato, wave 0): el pádel real es 2v2. El juego soporta **1v1**, **2v2**
> y un modo **Playground** (sandbox sin puntaje). Esta sección es el contrato; la implementación real
> (separación de cápsulas entre compañeros, IA llenando los slots que el cliente no controla) es
> wave 1.

### 19.1 `Mode` y cuántos jugadores

```go
type Mode uint8
const (
    Mode1v1      Mode = iota // 2 jugadores, 1 por equipo (hoy, default)
    Mode2v2                  // 4 jugadores, 2 por equipo (pádel real)
    ModePlayground           // sandbox libre: sin puntaje, pelota respawneable a demanda
)
const MaxPlayers = 4 // tamaño fijo de State.Players para TODOS los modos
```

`State.NumPlayers` (2 o 4) dice cuántos de los `MaxPlayers` slots están activos; `Match.Step` siempre
recibe `[MaxPlayers]PlayerInput` (los slots no usados se ignoran). `State` sigue siendo un array fijo
(`[MaxPlayers]Player`, no slice) para seguir siendo comparable con `==` y copiable sin allocs.

### 19.2 Convención de índices equipo/jugador

- `Player.Team` (reemplaza el viejo `Player.Side`): 0 = mitad X negativa, 1 = mitad X positiva.
- `Player.Slot`: 0 = compañero derecho, 1 = compañero izquierdo (sólo relevante en 2v2; posición de
  arranque/referencia de cajón de saque, **no** confina el movimiento en wave 0).
- Convención fija de `teamPlayers(team)`: equipo 0 = índices `{0,1}`, equipo 1 = índices `{2,3}`.

### 19.3 Reglas por EQUIPO (no por jugador)

- **Un toque por equipo por cruce**: cualquiera de los dos compañeros puede golpear, pero sólo cuenta
  **un** toque total — si el compañero que no golpeó también toca la pelota en el mismo cruce, es doble
  toque (falta), igual que si el mismo jugador la tocara dos veces.
- `RallyState.LastKicker` → **`RallyState.LastTeam`** (mismo rol, ahora nombrado por equipo) +
  **`RallyState.LastPlayer`** (qué jugador específico, dentro de ese equipo, tocó — informativo /
  para wave1, no participa hoy en la decisión de punto, que sigue siendo 100% a nivel equipo).
- `Score` sigue siendo `[2]int`, indexado por equipo — sin cambios.
- `Event.Side` sigue significando **equipo** (0/1), nunca un índice de `State.Players`. Se agrega
  **`Event.Player`** (índice en `State.Players`, o `-1` si el evento no involucra a un jugador
  específico) para que wave1 pueda saber *quién* exactamente tocó, sin romper la semántica de `Side`.

### 19.4 Separación de cápsulas entre compañeros

**Stub en wave 0, implementación en wave 1.** `movePlayers` hoy no evita que dos compañeros del mismo
equipo se superpongan (sólo los confina a su mitad de cancha en X/Z). La resolución de colisión
cápsula-cápsula entre compañeros es responsabilidad de `player.go` (wave1, junto con salto/stamina).

### 19.5 Orden de saque en 2v2

**Decisión del coordinador: documentar la elección.** Pádel/tenis "real" rota el saque entre los 4
jugadores juego a juego (A1, B1, A2, B2, ...), lo cual requiere un modelo de *games* que este código
no tiene (hoy se juega a puntos directos, `PointsToWin`). La regla elegida para wave 0, más simple y
sin ese modelo:

```go
// serverPlayerIndex: alterna compañero cada vez que el EQUIPO vuelve a sacar.
func serverPlayerIndex(s *State, team int8) int8 {
    if s.NumPlayers < 4 { return team } // 1v1: un jugador por equipo
    a, b := teamPlayers(team)
    if s.LastServerPlayer[team] == a { return b }
    return a
}
```

`State.LastServerPlayer [2]int8` recuerda, por equipo, qué jugador sacó la última vez que ese equipo
sacó. **Nota para wave1/2**: si se quiere la rotación real de dobles (round-robin de los 4 jugadores
a través de *games*), hace falta modelar *games*/*sets* explícitamente; hasta entonces esta alternancia
simple es la que corre.

### 19.6 Playground

- No se puntúa nunca: `applyRules` devuelve temprano (no-op) si `Cfg.Mode == ModePlayground`.
- `NewMatch` arranca directo en `PhaseRally` (sin fase de saque: no hay reglas de saque que cumplir).
- `(*Match).RespawnBall()` repone la pelota al centro de la cancha a demanda (botón del panel dev,
  wave1) — primitiva únicamente; wave 0 no cablea ningún trigger de UI.

### 19.7 Snapshot y wasm

- El snapshot agrega (al final, §11.2/más abajo) bloques de posición+extras para los jugadores 2 y 3,
  más `SnapNumPlayers` y `SnapMode`.
- El buffer de input de wasm pasa a `MaxPlayers × inputPerPlayer` (4×6=24 floats): el cliente llena los
  slots humanos; los slots de IA (si los hay) se llenan dentro de wasm en wave1 — hoy quedan en cero.

---

## 20. Contrato congelado (wave 0)

Layout final tal como lo implementa el código de `sim`/`wasm` al cierre de la wave 0. Referencia para
el agente de `client/` al portar `snapshot.ts`/`input.ts`/`engine.ts`.

### 20.1 `PlayerInput` (wasm input buffer)

`inputPerPlayer = 6`: `[moveX, moveZ, targetX, targetZ, buttons, spin]`.
`inputLen = MaxPlayers * inputPerPlayer = 24` floats, un bloque de 6 por cada uno de los 4 slots fijos
(1v1 y Playground sólo llenan los slots 0 y 1; 2v2 llena los 4).

### 20.2 `Event` (wasm event stride)

`eventStride = 14`: `[kind, tick, posX, posY, posZ, side, player, part, quality, velX, velY, velZ,
spin, _pad]`. `side` es el EQUIPO (0/1); `player` es el índice en `State.Players` o `-1`.

`EventKind` (valores viejos nunca cambian; nuevos sólo se agregan al final):

```
1 EvBounceFloor  2 EvHitWall  3 EvHitNet  4 EvKick  5 EvPoint  6 EvGameOver  7 EvOut
8 EvWhiff  9 EvJump  10 EvBodyHit  11 EvToss  12 EvServeFault  13 EvLet
```

### 20.3 Snapshot (`sim.Snap*`, wasm `outPrev`/`outCurr` blocks)

Índices viejos (0..14, hasta `SnapLastTeam` inclusive, antes `SnapLastKicker`) **sin cambios de
posición**. A partir de ahí, todo apendeado:

```
0 SnapTick  1 SnapPhase  2 SnapServer  3 SnapScore0  4 SnapScore1
5 SnapBallX 6 SnapBallY 7 SnapBallZ
8 SnapP0X 9 SnapP0Y 10 SnapP0Z  11 SnapP1X 12 SnapP1Y 13 SnapP1Z
14 SnapLastTeam                                    -- (antes: SnapLastKicker, mismo índice)

-- apendeado, contrato v2 + 2v2 --
15 SnapBallSpin  16 SnapBallTossed  17 SnapNumPlayers  18 SnapMode
19..25 SnapP0{VelX,VelZ,Stamina,Charge,OnGround,Buffer,Flags}
26..32 SnapP1{VelX,VelZ,Stamina,Charge,OnGround,Buffer,Flags}
33..35 SnapP2{X,Y,Z}  36..42 SnapP2{VelX,VelZ,Stamina,Charge,OnGround,Buffer,Flags}
43..45 SnapP3{X,Y,Z}  46..52 SnapP3{VelX,VelZ,Stamina,Charge,OnGround,Buffer,Flags}
53 SnapLen
```

`Charge` = `ChargeTicks / chargeMaxTicks` (0 si `Config` nunca se `Sanitize()`ó). `Flags` =
`ShotType | (StaminaLock>0)<<1`.

### 20.4 Config live-tuning (`sim/config_layout.go`)

`CfgScalarLen` = 45 escalares (orden de declaración de `Config`, incluye `Mode` al final) +
`PartCount(7) × cfgPartFieldCount(11)` = 77 floats de la tabla de partes (`HMin, HIdeal, HMax, Reach,
SpeedMin, SpeedMax, DevBase, DevMax, SpinMin, SpinMax, AllowLob∈{0,1}`, por cada una de las 7
`BodyPart`, incluyendo el índice 0 `PartNone` sin usar). `CfgLen = CfgScalarLen + 77 = 122`.

### 20.5 Lo que el doc original tenía y este wave 0 decidió distinto (para que el agente de wave 1 no se sorprenda)

1. **Saque**: autopase → drop + pique obligatorio + cajón diagonal + 2 saques (§7.5, override del
   coordinador). `ServeTossSpeed` default pasó de 4.5 a 0.
2. **Modos/2v2**: no estaba en el doc original; se agregó completo (§19) por decisión del coordinador,
   incluyendo el rename `Player.Side` → `Player.Team`, `RallyState.LastKicker` → `LastTeam` (+
   `LastPlayer`), y `Event.Player`.
3. **`sanitizeInput`**: el doc original describe la firma como `(in, side)`; wave 0 la implementó como
   `(in, side, cfg)` porque el clamp de `TargetX/TargetZ` depende de `Config.TargetMinFromNet` /
   `TargetWallMargin`, que son tuneables en vivo — una firma pura de 2 argumentos no puede leerlos.
4. **`eventStride`**: el doc dejaba "13 (o 12; fijar)" sin decidir cuál. Wave 0 fijó **14** (13 campos
   + `Event.Player`, que no estaba contemplado antes de la decisión 2v2).
5. **Compat v1**: mientras `strike.go` no exista (wave 1), `applyKicks`/`performKick` en `physics.go`
   mapean el *release* de `BtnStrike`/`BtnLob` al viejo pipeline de patada (apuntando a `TargetX/Z` en
   vez de `AimX/AimZ`, potencia fija en vez de carga real). Esto es un shim documentado con
   `TODO(wave1)`, no el solver final de §7-§9.

### 20.6 API JS wave 1 (agente D: `ai/` + `wasm/`)

Todas las funciones devuelven `null` en éxito o un `string` de error (nunca tiran excepción).
Ninguna se llama en medio de un `step`: se aplican **entre frames**.

| Función | Descripción |
|---|---|
| `futenis.configure(mode, difficulty, pointsToWin, humanMask)` | Reconstruye el match. `mode`: 0=1v1, 1=2v2, 2=playground. `difficulty`: 0=Easy, 1=Normal, 2=Hard, 3=Pro (todos los bots). `pointsToWin`: 1..99. `humanMask`: bit *i* = slot *i* lo controla el cliente; los demás slots activos los maneja `ai.Bot` **dentro de `step`, antes de cada tick** (el cliente puede dejar esos bloques del input en cero). Default sin llamar: 1v1, `humanMask = 0b1111` (todo humano, como wave 0). Error → la sesión no cambia. |
| `futenis.reset()` | Reinicia el match conservando modo, puntos, dificultad, máscara y la `Config` tuneada. |
| `futenis.respawnBall()` | Sólo `mode=2` (playground); en otro modo devuelve error. |
| `futenis.getConfig(u8)` | Llena un `Uint8Array` de `futenis.configLen * 8` bytes (float64 LE, orden `sim/config_layout.go`, `CfgLen = 122`). |
| `futenis.setConfig(u8)` | Lee el mismo layout, rechaza NaN/Inf, aplica `ReadConfig` (+`Sanitize`) al estado actual. `Mode` y `PointsToWin` se **ignoran** (sólo se cambian con `configure`). |
| `futenis.configLen` | Número (= `sim.CfgLen`). |

Un solo cruce JS↔Go por frame en `step` se mantiene: los bots corren del lado Go.

**Bot (`ai/`)**: `ai.New(difficulty, slot) Bot`, `(*Bot).Think(s *sim.State, slot int) sim.PlayerInput`.
Desvío respecto de §12: el "tiempo de reacción" no es un ring de inputs retrasados sino una **creencia
de la pelota** que se resincroniza `ReactionTicks` después de que la pelota real diverge de lo predicho
(golpe/rebote inesperado) — el bot sigue jugando sobre información vieja, más humano y sin desfasar sus
propios botones. Errores humanos vía `sim.Hash64` (tick, slot, `Bot.Seed`). El chequeo de alcance usa
hoy `Config.KickReach` (shim wave 0); al aterrizar `strike.go` hay que pasarlo a `PartSpec.Reach`.
