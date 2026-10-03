# Personaje jugable — dirección de arte, rig, pipeline y animación

Estado: propuesta (diseño, sin código). Depende de `docs/design/contact-v2.md` (§6 partes, §7.4 cápsula,
§11 eventos/snapshot, §18 dirección de arte).

Regla madre (no negociable, viene de §18): **lo visual nunca afecta la física ni las reglas**. La sim
(`sim/`) decide contacto, parte, calidad y trayectoria. El personaje es un *títere* que el cliente mueve
para que lo que se ve coincida con lo que la sim ya decidió.

Convención de verificación: lo marcado **[verificar]** es un dato que no confirmé contra la fuente
primaria al escribir esto; hay que chequearlo antes de comprometer plata o tiempo.

---

## 1. Direcciones de estilo

Contexto de cámara: alta, detrás de la línea de fondo, mirando a lo largo de X. El jugador propio se ve
mayormente de espaldas y el rival de frente, chico (cancha de 20 m). Lo que importa en ese plano:

1. **Silueta legible a ~40-80 px de alto** (el rival, al fondo).
2. **Orientación** (para dónde mira) y **altura del cuerpo vs pelota** (¿le llega con la cabeza?).
3. **Identidad del lado/equipo** a golpe de vista (color dominante en torso).

Rocket League resuelve esto con siluetas simples, colores de equipo planos y FX fuertes; nosotros
copiamos el *principio*, no el estilo.

### A. "Toy athlete" — chunky estilizado (RECOMENDADA)

- Proporciones ~5.5-6 cabezas (realista es ~7.5). Cabeza algo grande, manos y botines grandes,
  piernas de longitud casi realista (las piernas son la mecánica del juego, no se pueden achicar).
- Formas low-poly con bordes biselados, *flat/gradient shading* con paleta (estilo Schedule I /
  Quaternius): sin texturas pintadas, color por material/atlas de paleta.
- Por qué: cabeza y botines grandes = las dos partes que más tocan la pelota se leen desde lejos;
  el torso ancho y plano es lienzo ideal para color de equipo y skins de camiseta.
- Riesgo: cabeza grande "miente" sobre la altura de cabeza → se resuelve en §2 (anclamos el *centro*
  de la cabeza a la franja de la sim, no el tope).

### B. "Chibi" — 3-4 cabezas

- Muy vendible para skins (estilo Fall Guys/Stumble), animaciones exageradas baratas.
- **Descartada para MVP**: con 3-4 cabezas las piernas quedan cortas y la franja Knee (0.35-0.95 m)
  y Chest (0.95-1.40 m) no tienen correspondencia visual creíble con un cuerpo de 1.8 m. El jugador
  vería "le pegó con el pecho" en la panza. Rompe la lectura de la mecánica central.

### C. "Graphic sport" — proporción atlética + outline/toon

- 7 cabezas, *toon shading* con contorno (inverted hull), estética de ilustración deportiva.
- Más fiel a la mecánica, pero el outline duplica draw calls/vértices por personaje y la proporción
  realista se lee peor a 50 px. Buena opción para *más adelante* como línea de skins "premium" si el
  shader se comparte.

**Recomendación: A.** Es la que mejor balancea legibilidad, costo de producción y compatibilidad con
assets CC0 existentes (Quaternius/KayKit están en esa familia visual).

---

## 2. Proporciones atadas a la hitbox de la sim

El modelo se construye **a la escala de la sim**: 1 unidad = 1 m, pies en Y=0, alto total 1.80 m
(`PlayerHeight`), ancho de hombros ≤ 2×`PlayerRadius` = 0.70 m. Ninguna skin puede salir de la
cápsula de §7.4 salvo accesorios cosméticos chicos (pelo, gorra) — y esos **nunca** se usan para
nada lógico.

Las franjas de §6 están en `hRel` (altura de la pelota sobre los pies, parado). Los landmarks del
modelo en T-pose/idle deben caer así:

| Landmark (hueso) | Altura objetivo (m) | Franja sim §6 (hMin–hIdeal–hMax) | Nota |
|---|---|---|---|
| Empeine / punta del botín (`*ToeBase`) | 0.05–0.12 | Foot 0.00–0.20–0.45 | el 0.20 ideal es pelota a media altura de canilla: la pierna se levanta, ver §5 IK |
| Rodilla (`*Leg`) parado | 0.50 | Knee 0.35–0.60–0.95 | el ideal 0.60 corresponde a rodilla levantada (muslo ~30°); la animación de "knee" la sube a 0.6-0.9 |
| Centro del pecho (`Spine2`) | 1.20 | Chest 0.95–1.20–1.40 | **exacto**: el esternón del modelo en 1.20 |
| Hombro (`*Shoulder`/`*Arm`) | 1.48–1.50 | Shoulder 1.30–1.50–1.65 | |
| Centro de cabeza (`Head` + ~0.1) | 1.65–1.70 | Head 1.60–1.85–2.10 | tope de cabeza 1.80; el ideal 1.85 implica estirarse/saltar → la animación de cabeceo eleva cuello y talones |
| Bicycle | — | 1.70–2.10–2.50 | sólo en el aire; lo resuelve la animación con root en Y real del snapshot |

Reglas de construcción:

- Con estilo A (cabeza grande) la cabeza crece **hacia abajo y a los lados**, no hacia arriba: el tope
  queda en 1.80 y el cuello se acorta. Así el centro visual de la cabeza queda en la franja Head.
- La cintura queda en ~1.0 m (borde Knee/Chest) — es el "hueco cerrado" 0.95 de §6; visualmente el
  cinturón/short marca la frontera entre "le pegó con la rodilla" y "con el pecho".
- **Test de aceptación del asset** (automatizable en el loader en `?dev=1`): leer las posiciones
  world de los huesos en bind pose y comparar contra la tabla con tolerancia ±0.05 m; warn en consola
  si no cumple. Dibujar la cápsula de hitbox (ya previsto en §11.4) superpuesta.

Las constantes de altura del cliente se leen de `getConfig()` (§11.3/§13), **no se duplican** en TS:
si se tunea la tabla de partes, el validador lo detecta.

---

## 3. Rig único compartido y cosméticos

### 3.1 Esqueleto

Un solo esqueleto humanoide, nombres **compatibles con Mixamo** para poder retargetear animaciones
de Mixamo y de packs que ya usan esa convención. Mixamo exporta con prefijo `mixamorig:`; el
`GLTFLoader` de three.js sanitiza nombres de nodo (`PropertyBinding.sanitizeNodeName` elimina `:` y
otros caracteres reservados) → en runtime quedan como `mixamorigHips` **[verificar en 0.169]**.
Decisión: en Blender renombrar a **sin prefijo** (`Hips`, `Spine`, ...) y mapear al importar clips.

Lista mínima (~24 huesos; < 30 mantiene el skinning barato):

```
Hips (root de animación; el root de mundo es el Object3D del jugador)
├─ Spine ─ Spine1 ─ Spine2 ─ Neck ─ Head ─ (HeadTop_End: socket gorra/pelo)
│                        ├─ LeftShoulder ─ LeftArm ─ LeftForeArm ─ LeftHand
│                        └─ RightShoulder ─ RightArm ─ RightForeArm ─ RightHand
├─ LeftUpLeg ─ LeftLeg ─ LeftFoot ─ LeftToeBase ─ (LeftToe_End)
└─ RightUpLeg ─ RightLeg ─ RightFoot ─ RightToeBase ─ (RightToe_End)
```

Sin dedos (manos tipo mitón en estilo A → 0 huesos de dedos, ahorra ~30 huesos vs Mixamo full).
Huesos extra opcionales (sockets, no deformantes): `BallContact_Chest`, `BallContact_Head` como
targets visuales de IK si hace falta.

### 3.2 Skin = mesh + material, nunca esqueleto

- Todas las piezas son `SkinnedMesh` bindeadas al **mismo** `Skeleton` (mismos nombres y bind pose).
- Una skin es un manifiesto JSON que elige piezas por slot:

| Slot | Ejemplo | Regla |
|---|---|---|
| `body` | base de piel/brazos/piernas | obligatoria, define tono de piel por paleta |
| `head` | cara | se ancla a `Head` |
| `hair` | pelo/gorra | rígido o pesado 100% a `Head` (sin física de pelo en MVP) |
| `shirt` | camiseta | color de equipo en zona dominante (ver 3.3) |
| `shorts` | short | |
| `boots` | botines | alto contraste con el piso: la parte que más se mira |
| `fx` (BP) | trail, aura de golpe perfecto | no-mesh; partículas por evento |

- Para evitar "z-fighting" entre body y ropa: la pieza `body` viene por variantes con zonas ocultas
  eliminadas (torso sin geometría donde hay camiseta), o se usa *body masks* por slot. MVP: body
  cortado por regiones (brazos, piernas, cuello) y la ropa provee el torso.
- Merge en runtime: después de elegir piezas, mergear las que comparten material en una sola
  `SkinnedMesh` (BufferGeometryUtils.mergeGeometries respeta atributos de skin si todas comparten
  esqueleto **[verificar]**) → 1-2 draw calls por personaje.

### 3.3 Paleta / atlas

- **Una textura de paleta compartida** (p. ej. 256×256, celdas de color planas + algunos gradientes)
  y los UV de cada pieza apuntan a celdas. Cambiar de color = cambiar UV offset o la textura de
  paleta, no el modelo. Es el truco estándar de los packs low-poly (Quaternius/KayKit usan
  atlas de paleta **[verificar por pack]**).
- **Color de equipo**: celdas reservadas `team_primary` / `team_secondary` que el shader sustituye
  por uniform (`onBeforeCompile` sobre `MeshStandardMaterial`/`MeshToonMaterial`) → la skin del
  rival siempre se ve con el color de su lado, sin importar qué compró. Esto protege la legibilidad
  (Rocket League hace lo mismo: el auto lleva color de equipo aunque tenga skin).
- Skins premium pueden traer una textura extra chica (emissive/patrón 256²) sobre la misma UV.

---

## 4. Pipeline: opciones y tradeoffs

### (a) Bases CC0: Quaternius / KayKit

- **Quaternius**: modelos low-poly, licencia **CC0** declarada en quaternius.com (uso comercial libre,
  sin atribución). Tiene packs de personajes animados ("Ultimate Animated Character Pack", "Universal
  Animation Library" **[verificar nombres actuales]**).
- **KayKit (Kay Lousberg)**: packs de personajes estilizados, **CC0** según la página de itch.io de
  cada pack **[verificar por pack: la mayoría CC0, algunos bundles pagos]**.
- Pro: gratis, rigged, estilo coherente con dirección A, cero riesgo legal con CC0.
- Contra: todo el mundo los usa → **cero identidad**; proporciones no coinciden con la tabla §2
  (hay que reescalar huesos); rigs propios (no siempre nombres Mixamo).
- Uso: **sólo prototipo y referencia de proporciones**, no producto final.

### (b) Animaciones Mixamo (Adobe)

- Gratis con cuenta Adobe. Según las FAQ de Mixamo, personajes y animaciones son *royalty free* para
  uso personal, comercial y sin fines de lucro, incluidos juegos; **no** se permite redistribuirlos
  como assets sueltos (ej. venderlos/publicarlos como pack) **[verificar texto vigente en
  helpx.adobe.com/creative-cloud/faq/mixamo-faq.html]**.
- En un juego web, el GLB con las animaciones viaja al cliente: es "distribuir como parte del juego",
  que es el caso permitido; igual conviene embeberlas en nuestro GLB (no servir los FBX originales).
- Pro: cientos de clips (correr, saltar, patear, cabecear), auto-rigger.
- Contra: catálogo genérico (no hay "chilena de futtenis" ni "rodilla en pared"); mocap realista que
  choca con estilo A → hay que exagerar en Blender; dependencia de un servicio de Adobe que puede
  cambiar términos.
- Uso: **locomoción y saltos** desde MVP; strikes como base para retocar.

### (c) Generación 3D con IA (Meshy, Tripo, Higgsfield `generate_3d`)

- Flujo: concept 2D (también puede ser IA) → imagen a GLB → **retopología manual** (las mallas IA
  salen trianguladas densas, sin loops de deformación) → UV a paleta → rig con nuestro esqueleto.
- Licencias: Meshy indica que en plan gratis los assets son CC BY 4.0 y en planes pagos el usuario
  tiene la propiedad **[verificar términos vigentes]**; Tripo y Higgsfield tienen términos propios
  **[verificar]**. Además: incertidumbre jurídica general sobre copyright de outputs IA → no basar el
  personaje *héroe/marca* sólo en IA.
- Pro: rapidísimo para explorar silueta y para **volumen de cosméticos** (gorras, peinados, botines).
- Contra: topología inutilizable sin retopo, estilo inconsistente entre piezas, el rig hay que
  hacerlo igual.
- Uso: **ideación y blockout de cosméticos**, nunca directo a producción.

### (d) Custom en Blender

- Modelar la base sobre la plantilla de alturas de §2, rig propio con nombres de §3.1, exportar glTF.
- Pro: identidad, control total de proporciones/topología/presupuesto, propiedad clara.
- Contra: tiempo (un artista con experiencia: días para la base, horas por cosmético) **[estimación]**.
- Es el destino final inevitable para un juego que vende skins.

### Camino recomendado por fases

| Fase | Personaje | Animación | Objetivo |
|---|---|---|---|
| **0. Prototipo** (ya) | cápsula + esferas por parte | ninguna | validar sim |
| **1. Prototipo jugable** | base CC0 (Quaternius/KayKit) reescalada a §2, renombrada a §3.1 | Mixamo: idle/run/sprint/jump + 3 strikes | probar AnimationMixer + IK + eventos; medir budgets |
| **2. Vertical slice** | base **custom Blender** estilo A + 2 skins + 5 cosméticos (blockout con IA, retopo manual) | Mixamo retocado + strikes custom por parte (6) | look final, pipeline de skins probado de punta a punta |
| **3. Producción** | catálogo de cosméticos por temporada (pase de batalla) | clips de celebración/emotes custom | escalar: el costo marginal de una skin = 1 mesh + paleta |

---

## 5. Sistema de animación manejado por la sim

Entradas (todas ya existen o están previstas en contact-v2 §11.2/§11.1): `Pos` (incl. Y real),
`VelX/VelZ`, `OnGround`, `Charge`, `Buffer`, `Flags` (ShotType), `Stamina`, y eventos `EvKick`
(`Part`, `Quality`, `Pos` del contacto, `Vel`, `Spin`), `EvBodyHit`, `EvWhiff`, `EvJump`, `EvToss`.

Principio: **el animador es una función pura de (snapshot interpolado, eventos recientes, tiempo de
render)**. No lee input, no guarda estado de juego, no tiene `if` de reglas.

### 5.1 Capas

1. **Locomoción** (`AnimationMixer`, blend por peso): `idle`, `run`, `sprint`, `strafe_L/R`,
   `backpedal`. Peso por `|vel|` y ángulo entre velocidad y orientación (orientación = hacia el target
   o el balón; decisión de cliente). `timeScale` del clip proporcional a velocidad para evitar
   *foot sliding*.
2. **Aire**: `jump_start` en `EvJump`, `air_loop` mientras `!OnGround`, `land` al volver a `OnGround`.
   La altura la pone la sim (root Y = `Pos.Y`); el clip **no** tiene root motion vertical.
3. **Anticipación** (preparación del golpe): mientras `Charge > 0`, mezclar la pose "wind-up" de la
   parte *probable* (la que corresponde a la altura predicha de la pelota; el cliente puede usar el
   mismo predictor de la IA §12 sólo para visual) con peso = `Charge`. Con `Buffer` activo, sostener la
   pose. Si el golpe no llega → `EvWhiff` dispara `whiff_<part>`.
4. **Golpe**: en `EvKick`, `clipAction('strike_' + part)` con `crossFadeFrom` corto (~60-80 ms). Un clip
   por parte: `strike_foot`, `strike_knee`, `strike_chest`, `strike_shoulder`, `strike_head`,
   `strike_bicycle`. Variante por calidad: mismo clip, `timeScale`/amplitud distintos o un clip
   `_forced` torpe.
5. **IK de contacto** (ver 5.2), encima de todo.
6. **Reacciones**: `EvBodyHit` → `flinch` aditivo (AnimationUtils.makeClipAdditive).

### 5.2 IK: que el pie/cabeza toque la pelota donde la sim dijo

El evento llega **después** del contacto (la sim ya resolvió en el tick N). Para que se vea bien:

- **Clips autorizados con el frame de impacto marcado** (metadata `impactTime` por clip). Al recibir
  `EvKick` en tick N, se arranca el clip *adelantado* a `impactTime − pequeño margen`: lo que se ve
  es el final del swing + follow-through. Con la anticipación de 5.1.3 la transición no salta.
- **Corrección IK** sobre la cadena de la parte: `CCDIKSolver` de three.js
  (`three/addons/animation/CCDIKSolver.js`, opera sobre `SkinnedMesh` con lista de `iks`:
  `target`, `effector`, `links` con límites de rotación). Target = `EvKick.Pos` (convertido a espacio
  del personaje). Cadenas:
  - Foot: `UpLeg → Leg → Foot` (effector `ToeBase`), límite de rodilla 0..150° en un eje.
  - Knee: effector `Leg`, cadena `Hips(rot) → UpLeg`.
  - Head: `Spine2 → Neck → Head` (effector `HeadTop_End` o socket frontal).
  - Chest/Shoulder: rotación de `Spine1/Spine2` para orientar el pecho/hombro (no IK completo).
- Peso del IK con curva campana centrada en el impacto (0 → 1 → 0 en ~150 ms). Si la distancia al
  target supera el alcance de la cadena (+ tolerancia), **no** forzar: se ve un *near-miss*; no
  importa, la pelota ya sale con la trayectoria de la sim.
- Lo **único** que se dibuja con verdad física es la pelota (posición de snapshot). El cuerpo se
  ajusta a la pelota, nunca al revés.

### 5.3 Feedback por calidad (`QualPerfect`/`QualGood`/`QualForced`)

| Calidad | Hit-stop | Cámara | FX | Animación |
|---|---|---|---|---|
| Perfect | 60-80 ms de *render* congelado (personaje + pelota visual) | shake corto + micro zoom | flash + trail cosmético (slot `fx`) | clip completo, follow-through amplio |
| Good | 0-30 ms | — | chispa chica | normal |
| Forced | 0 | — | polvo/torpe | `_forced`, timeScale bajo, desbalance |

**Hit-stop no pausa la sim**: el acumulador de ticks sigue; sólo el render aplica un *delay* visual
a la pelota y al personaje y luego recupera (catch-up rápido en ~100 ms). En online, esto es
puramente local. Sonido y vibración (gamepad) por calidad y parte.

### 5.4 Fuera de alcance de 5

Root motion horizontal (la sim mueve el cuerpo), ragdoll, física de tela/pelo, *motion matching*.

---

## 6. Presupuestos para web

Escenario: 2 jugadores en MVP (1v1), presupuesto calculado para **4** (2v2 futuro / espectadores /
pantalla de selección).

| Ítem | Presupuesto | Nota |
|---|---|---|
| Triángulos por personaje LOD0 | 6k–10k (todas las piezas) | estilo A low-poly; cámara lejana |
| LOD1 (rival al fondo / mobile) | 2k–4k | `THREE.LOD` o elegir por calidad gráfica |
| Huesos | ≤ 30 | skinning por GPU; sin dedos |
| Influencias por vértice | ≤ 4 | límite estándar glTF `JOINTS_0/WEIGHTS_0` |
| Draw calls por personaje | 1–2 (merge por material) + 1 sombra blob | 4 jugadores ≤ ~10 DC |
| Textura paleta | 256² compartida por todo | |
| Textura premium por skin | ≤ 512², KTX2 (Basis UASTC/ETC1S) | `KTX2Loader` + transcoder basis |
| Geometría | glTF binario (GLB) con **meshopt** (`EXT_meshopt_compression`) o Draco | meshopt descomprime más rápido y comprime también animación; Draco no comprime animación **[verificar]** — preferir meshopt |
| Animaciones | GLB aparte `anims.glb`, compartido por todas las skins | cuantizar con `gltf-transform` (`resample`, `quantize`) |
| Peso de carga inicial (personajes) | **≤ 1.5 MB** (base + 2 skins + anims) | assets de skins extra se bajan lazy |
| Peso por cosmético extra | ≤ 100–200 KB | |

Herramientas: `gltf-transform` CLI (es Node; corre **en el pipeline de assets**, no en el build del
juego — no viola "sin Node" del runtime; alternativa sin Node: `gltfpack` de meshoptimizer, binario
nativo) **[decisión abierta §8]**. Decoders: `GLTFLoader.setMeshoptDecoder(MeshoptDecoder)`,
`setDRACOLoader`, `setKTX2Loader` (addons de three; servir los .wasm/.js del decoder desde la misma CDN
del import map, versión fija 0.169.0).

---

## 7. Layout de archivos, loader y tareas

### 7.1 Archivos

```
assets-src/characters/            # fuentes (.blend, concept). Fuera de client/, opcional git-lfs
client/assets/characters/
  rig/skeleton.glb                # esqueleto + body base (bind pose de referencia)
  anims/anims.glb                 # todos los clips, nombres estables (idle, run, strike_foot, ...)
  parts/head/<id>.glb
  parts/hair/<id>.glb
  parts/shirt/<id>.glb
  parts/shorts/<id>.glb
  parts/boots/<id>.glb
  palettes/base.ktx2              # paleta compartida (o .png en fase 1)
  skins/<skinId>.json             # manifiesto: { slots: {head, hair, shirt, shorts, boots}, palette, fx }
  manifest.json                   # índice de skins/partes + hashes para cache
```

### 7.2 Módulos de cliente

```
client/src/character/
  loader.ts        # carga skeleton+anims una vez; parts on-demand con cache por URL; configura decoders
  assemble.ts      # (skin manifest) -> Object3D: bindea piezas al Skeleton compartido, merge por material
  validate.ts      # ?dev=1: compara alturas de huesos vs getConfig() (§2); dibuja cápsula
  animator.ts      # Animator: update(snapInterp, events, dt) -> capas de §5.1 (AnimationMixer)
  ik.ts            # cadenas CCDIKSolver por parte + curva de peso (§5.2)
  feedback.ts      # hit-stop visual, shake, FX por Quality (§5.3)
  teamColor.ts     # onBeforeCompile: celdas team_* -> uniform
```

Contrato: `render.ts` deja de dibujar cápsulas y pide `CharacterView` por jugador:
`view.update(interpSnapshot, eventsThisFrame, renderDt)`. Ningún módulo de `character/` importa
`wasm` ni escribe input. Clonado de personajes skinneados: `SkeletonUtils.clone` (addons) para no
compartir instancias de hueso entre jugadores.

### 7.3 Tareas para agentes en paralelo

Ola 1 (independientes):
1. **T-char-1 assets proto**: conseguir base CC0 (Quaternius o KayKit), reescalar a §2, renombrar
   huesos a §3.1, exportar `skeleton.glb`; registrar licencia en `client/assets/LICENSES.md`.
2. **T-char-2 anims proto**: bajar de Mixamo idle/run/sprint/jump/land + 3 strikes, retarget al rig,
   marcar `impactTime` en extras, empaquetar `anims.glb` con meshopt.
3. **T-char-3 loader**: `loader.ts` + `assemble.ts` con decoders y cache; test con un GLB dummy.
4. **T-char-4 validate**: `validate.ts` + overlay de cápsula/franjas en `?dev=1`.

Ola 2 (dependen de 1-3):
5. **T-char-5 animator**: capas locomoción/aire/anticipación/golpe desde snapshot+eventos.
6. **T-char-6 ik**: cadenas CCD por parte + curva de peso; escena de prueba con pelota fija a cada hIdeal.
7. **T-char-7 feedback**: hit-stop visual sin tocar el acumulador de la sim, shake, FX por calidad.
8. **T-char-8 teamColor + paleta**.

Ola 3: 9. integración en `render.ts` + medición (tris, DC, MB) contra §6. 10. Vertical slice: modelo
custom en Blender (trabajo humano/artista; los agentes preparan la plantilla de alturas y el checklist).

Criterio global: `go test ./sim/...` intacto (no se toca `sim`), cero imports de `character/` desde
`sim`/`wasm`.

---

## 8. Decisiones abiertas

1. **Estilo**: A "toy athlete" (5.5-6 cabezas) vs C atlético toon. *Recomiendo A*; C queda como línea
   premium futura.
2. **Base del prototipo**: Quaternius vs KayKit. *Recomiendo* el que tenga piernas más largas y rig más
   cercano a Mixamo tras inspeccionarlos (tarea T-char-1 decide con evidencia); ambos CC0.
3. **Compresión**: meshopt vs Draco. *Recomiendo meshopt* (geometría + animación, decoder chico).
4. **Herramienta de pipeline de assets**: `gltf-transform` (Node, sólo offline) vs `gltfpack` (binario
   nativo). *Recomiendo `gltfpack`* para respetar "sin Node" también en tooling; `gltf-transform` sólo si
   falta una feature.
5. **IA para cosméticos**: usarla para blockout de piezas (con retopo manual) o no usarla. *Recomiendo
   sí, sólo blockout*, en plan pago con términos de propiedad verificados, y nunca para el personaje
   base/marca.
