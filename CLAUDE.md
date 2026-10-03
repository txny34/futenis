# futenis — Rules del proyecto

Futtenis (fútbol-tenis) en cancha de pádel con paredes. 1v1. **Máximo 1 toque por lado.**
Juego de navegador, meta comercial (online, skins, pase de batalla) — pero **ingeniería primero, arte después**.

## Arquitectura (NO NEGOCIABLE)

```
sim/      Go puro. Reglas + física. Fuente de verdad. CERO dependencias de JS/DOM/red.
wasm/     package main (GOOS=js). Único puente sim <-> navegador.
cmd/dev/  Servidor de desarrollo en Go (estáticos + build). Sin Node.
cmd/server/ (futuro) Servidor autoritativo: corre el MISMO sim nativo.
client/   TypeScript + three.js. SOLO dibuja, lee input y maneja cámara. NUNCA decide reglas.
```

- **Sin Node.** TS se compila con el binario `esbuild` (escrito en Go). three.js vía import map.
- La dependencia va en una sola dirección: `client -> wasm -> sim`. `sim` no conoce a nadie.

## Estándares de juego en browser

1. **Fixed timestep**: `sim` avanza en ticks de `1/60 s` (`TickRate`). El cliente acumula tiempo real
   con `requestAnimationFrame` y llama `step` N veces; renderiza **interpolando** entre el estado previo y el actual.
2. **Determinismo**: mismo estado + mismos inputs = mismo resultado, nativo y WASM.
   - Prohibido: `time.Now`, `math/rand` sin semilla del estado, iterar maps, goroutines en `sim`.
   - Expresiones float de más de una operación sobre la misma línea: forzar redondeo con `float64(...)`
     donde el compilador pueda fusionar en FMA (arm64). Usar los helpers de `vec.go`.
3. **Input como acciones**, no teclas: `sim.PlayerInput`. El mapeo teclado/joystick/táctil vive en `client`.
4. **Red (futuro)**: gracias a la regla de 1 toque, la trayectoria de la pelota entre toques es determinista.
   Se sincronizan **eventos de toque** + movimiento de jugadores, no la pelota frame a frame.
   Servidor autoritativo, predicción en cliente, interpolación de rivales.

## Reglas WASM (performance)

- **Un solo cruce JS<->Go por frame** para el estado: snapshot en buffer fijo (`[]float64` → bytes LE),
  layout definido en `sim/snapshot.go`. Prohibidas llamadas granulares por entidad.
- **Cero allocations por tick en `sim`**: buffers pre-allocados, `events` se reutiliza (`[:0]`).
- Medir tamaño del `.wasm`; si molesta, evaluar TinyGo.

## Mundo

- Unidades: metros, segundos. **Y arriba**. X = largo (-10..10), Z = ancho (-5..5). Red en X=0.
- Lado 0 = X negativo, lado 1 = X positivo.

## Calidad

- TDD en `sim`: test primero. `go test ./sim/...` debe pasar siempre. Coverage ≥ 80% en `sim`.
- `go vet ./...` limpio. Funciones < 50 líneas, archivos < 400.
- Inmutabilidad donde no cueste performance; en el hot path de `sim` se muta in-place a propósito (documentado).
- Commits convencionales (`feat:`, `fix:`, ...). Sin atribución de IA.

## Fuera de alcance por ahora (YAGNI)

Modelos 3D, skins, menús, React, ECS, base de datos, pagos, servidor online. Llegan por etapas.
