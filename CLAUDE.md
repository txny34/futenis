# futenis — Rules del proyecto

Futtenis (fútbol-tenis) en cancha de pádel con paredes. **1v1 y 2v2** (+ Playground). **Un toque por equipo por cruce.**
Reglas = **reglamento de pádel** adaptado al pie (saque, cuadros, let, doble falta, 15/30/40, juegos, sets).
Juego de navegador, meta comercial (online, skins, pase de batalla) — **ingeniería primero, arte después**.

**Pilar de diseño:** fácil de aprender, dificilísimo de dominar. **Referencia: Rocket League** — pocos inputs,
física continua, mecánicas emergentes, recursos visibles (stamina = turbo), transferencia de momento.

## Arquitectura (NO NEGOCIABLE)

```
sim/      Go puro. Reglas + física + puntuación + predicción. Fuente de verdad. CERO dependencias de JS/DOM/red.
ai/       Go puro. Bot determinista (4 dificultades, 2v2). Produce sim.PlayerInput, como un humano. Sin trampas.
wasm/     package main (GOOS=js). Único puente sim/ai <-> navegador. Corre el bot en los slots no humanos.
cmd/dev/  Servidor de desarrollo en Go (estáticos + build con esbuild Go API). Sin Node. Default :8090 (WSL2).
game/     Go puro. Sesión (modo, config, bots) + formato de frame. Compartido por wasm/ y cmd/server/.
cmd/server/ Servidor online autoritativo (docs/design/online-spike.md): MISMO sim + ai nativos. Salas por código +
            cola de matchmaking (`/ws?queue=1v1`, queue.go). UNA sola máquina (estado en memoria).
client/   TypeScript + three.js. SOLO dibuja, lee input, cámara, HUD, menú, feedback. NUNCA decide reglas.
docs/design/  contact-v2.md (diseño aprobado + contrato congelado §20), character.md (pipeline de personaje).
```

- **Sin Node.** TS se compila con la API Go de `esbuild`. three.js (+ `three/addons/`) vía import map.
- Dependencias en una sola dirección: `client -> wasm -> game -> (ai, sim)`, `cmd/server -> game`, `ai -> sim`. `sim` no conoce a nadie.

## Estándares de juego en browser

1. **Fixed timestep**: `sim` a **120 Hz** (`TickRate`, como la física de RL). Red a 60 Hz (futuro). Render sin tope,
   interpolando entre snapshot previo y actual. Duraciones en `Config` en **segundos**; el sim deriva ticks.
2. **Determinismo**: mismo estado + mismos inputs = mismo resultado, nativo y WASM.
   - Prohibido: `time.Now`, `math/rand` (usar `sim.Hash64`/splitmix sobre el estado), iterar maps, goroutines en `sim`/`ai`.
   - Expresiones float de más de una operación: forzar redondeo con `float64(...)` (FMA en arm64). Helpers de `vec.go`.
3. **Input como acciones** (`sim.PlayerInput` v2): `MoveX/Z`, `TargetX/Z` (punto en cancha rival), `Buttons`
   (Strike=1, Lob=2, Jump=4, Sprint=8, Dash=16) como **niveles mantenidos** — el sim detecta flancos. `Spin` −1/0/+1.
   Mapeo teclado/mouse en `client/src/input.ts` (WASD, mouse apunta, click izq/der, rueda, Espacio, Shift, Q).
4. **Golpe**: parte del cuerpo por altura (pie, rodilla, pecho, hombro, cabeza, chilena), buffer de timing con calidad
   (perfect/good/forced), desvío determinista, velocidad **analítica** resuelta contra el integrador real.
   Efecto solo en piques/paredes (sin Magnus) → trayectoria entre toques exacta.
5. **Red (futuro)**: se sincronizan **eventos de toque** + movimiento de jugadores, no la pelota frame a frame.

## Contrato y layout (CRÍTICO)

- Snapshot, eventos, input y config tienen layout congelado en `docs/design/contact-v2.md §20`. **Solo se agrega al final.**
- El cliente verifica el layout del wasm al cargar (`futenis.layout()` + `assertLayout`). Si cambiás un layout en Go,
  actualizá `client/src/snapshot.ts` / `engine.ts` / `dev/panel.ts` **en el mismo commit**.
- Toda config que entra por wasm pasa por `Config.Sanitize()` (rangos min/max en cada campo).

## Reglas WASM (performance)

- **Un solo cruce JS<->Go por frame** (snapshot en buffer fijo, bytes LE). Prohibidas llamadas granulares por entidad.
- **Cero allocations por tick** en `sim` y `ai` (`testing.AllocsPerRun` en los tests). Cero allocations por frame en el cliente.

## Mundo

- Metros, segundos. **Y arriba**. X = largo (-10..10), Z = ancho (-5..5). Red en X=0. Línea de saque ±6.95.
- Equipo 0 = X negativo (humano, cámara detrás), equipo 1 = X positivo. `MaxPlayers = 4` (array fijo).

## Arte y cliente

- Dirección: **"arena nocturna neón"** indie con identidad propia. Colores **solo** en `client/src/env/theme.ts` (tokens).
- Personaje **procedural** "toy athlete" (`client/src/character/`), hitbox del sim idéntico para todas las skins;
  lo visual nunca afecta la física. Rig compartido → skins = parámetros.
- Feedback tipo juego competitivo: anillo de timing, marcador de pique, flash dorado, callouts, VS / Victoria.

## Calidad

- TDD en `sim` y `ai`. `go test ./...` siempre verde. Coverage: `sim` ≥ 95%. Soak tests (`go test -run Soak ./sim/`).
- `gofmt` + `go vet ./...` + `GOOS=js GOARCH=wasm go vet ./wasm/` limpios. `-race` en CI local.
- Funciones < 50 líneas, archivos Go < 400, archivos TS < 200. TS strict, sin `any`.
- Review independiente (go-reviewer) por etapa: cero CRITICAL/HIGH antes de mergear.
- Commits convencionales (`feat:`, `fix:`, ...). Sin atribución de IA. Nunca push directo a `main`.
- Commit chico por cada cambio verificado (revisión rápida commit a commit).
- CI (`.github/workflows/ci.yml`) tiene que estar verde: fmt, vet, `-race`, coverage `sim`, build (`go run ./cmd/dev -check`),
  staticcheck, govulncheck, `tsc` strict, límites de tamaño (`scripts/ci/size-check.sh`).

## Al abrir un PR (obligatorio)

1. Correr un code review completo del diff contra `main` (go-reviewer + typescript-reviewer, y security-reviewer si toca
   red, wasm bridge, storage o input) contra las reglas de este archivo.
2. Corregir en la misma rama todo CRITICAL/HIGH y los MEDIUM claros, en commits chicos; re-verificar y pushear.
3. Esperar el CI verde y dejar un comentario en el PR con: qué se revisó, hallazgos, commits de arreglo y qué queda para humano.

## Correr

```
go run ./cmd/dev          # http://localhost:8090  (?dev=1 = panel de tuning)
                          # http://localhost:8090/character.html = visor del personaje
go run ./cmd/server       # ws://localhost:8091  ->  http://localhost:8090/online.html?room=ABCDE (2 pestañas; ?lag=150)
FUTENIS_SERVER=wss://futenis-server.fly.dev bash scripts/deploy/client.sh --prod --yes   # cliente -> futenis.vercel.app
~/.fly/bin/flyctl deploy --ha=false --remote-only                                       # server -> Fly.io gru (1 máquina)
go test -short ./...      # rápido;  sin -short incluye soak y harness del bot (lento)
bash scripts/ci/local.sh  # CI completo local (mismos pasos que ci.yml). OBLIGATORIO antes de cada push. STRESS=20 para flaky
```

## En curso: online-first (modal online, matchmaking, 2v2 online, lobby compartido, login Supabase Google/Discord, ranked con ELO en server)

## Fuera de alcance por ahora (YAGNI)

Predicción online y reconexión, pagos, skins comerciales, monedas, habilidades especiales,
flip en el aire → v3. React, ECS.
