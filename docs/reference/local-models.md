# Modelos locales y media — encaje con la arquitectura Go

Documento de profundidad ligado al borrador [ARCHITECTURE_LEGACY_ES.md](../archive/architecture-legacy-es.md) (**§2 ter**). Resume una estrategia **realista para 2025–2026** en PC tipo **RTX 4050 + 32 GB RAM** y cómo encaja en **nuestro** diseño (CLI/orquestador en **Go**, no Python). Hub actual: [architecture.md](../architecture.md); comportamiento GoClaw: [goclaw/CLAUDE.md](../../goclaw/CLAUDE.md).

---

## 1. Veredicto

- Construir un asistente tipo “**Claude Code lite**” con **LLM local** es **coherente** con el mapa del §3: el bloque `LLM_API` pasa a ser un **servicio en la misma máquina** (típicamente **Ollama** escuchando en `127.0.0.1`), y el cliente en Go habla **HTTP** igual que con un proveedor remoto.
- Lo que el resumen externo llama “stack CLI moderno” (**Typer + Rich + Prompt Toolkit**) es **Python**; en nuestro proyecto la “cara visible” sigue siendo **Go** (`internal/channel`: REPL simple o TUI con librería Go si se elige en D8). No hace falta renunciar a riqueza de terminal: hay equivalentes en el ecosistema Go.

### 1.1 Panorama open-weights (2026) — qué aprovechar y qué filtrar

Resúmenes “de internet” coinciden en lo global: modelos **open-weights** ganan terreno frente a APIs cerradas por **privacidad**, **coste** a largo plazo, **evitar lock-in** y **auditoría**; hay **especialización** (chat general vs **coder** vs razonamiento “largo” vs imagen/vídeo). Herramientas tipo **Ollama**, LM Studio o Jan encajan en “ejecución local”.

**Para nuestro diseño el filtro no es la lista de máximos del mercado, sino:**

1. **Hardware:** rankings que citan DeepSeek-V3, “Llama 4”, Qwen grandes, etc. suelen asumir **muchas GPU** o uso **API**. Con **RTX 4050 ~6 GB VRAM** (§2.5), el subconjunto útil son **variantes pequeñas/medias cuantizadas** que existan en [biblioteca Ollama](https://ollama.com/library) o equivalente — no el checkpoint más citado en Twitter.
2. **Tarea del producto:** prioridad **coding + tool-use** (§2.2, **D2**), no solo chat creativo; conviene **familias “coder”** y pruebas de **JSON / tools** reales, no solo benchmarks de conversación.
3. **Multimodal / generación de imagen:** modelos tipo **Flux** o pipelines vídeo van al apartado **§3** (herramientas opcionales), no sustituyen al LLM del orquestador.
4. **Complejidad operativa:** los límites que citan los resúmenes (despliegue vs API) son reales; por eso el doc fija **Ollama como proceso externo** y **D7** para config, en lugar de embarcar inferencia en el binario Go.

Los nombres concretos cambian cada mes; **D11** debe cerrar **un** identificador Ollama probado en tu PC, no una colección teórica.

---

## 2. LLM local (cerebro)

### 2.1 Runtime recomendado: Ollama

- **Por qué:** uso extendido en la comunidad, instalación sencilla, uso de **GPU** cuando hay drivers CUDA adecuados, API HTTP estable para integrar desde Go sin embarcar `llama.cpp` en el binario.
- **Rol en la arquitectura:** proceso **externo** al asistente Go; `internal/llm` implementa un cliente que apunta a `OLLAMA_HOST` (o configurable en D7).

### 2.2 Modelos recientes usados en local (codificación / asistente)

| Perfil | Modelo (orientativo) | Nota |
|--------|----------------------|------|
| Rápido, menos VRAM | Qwen2.5-Coder **7B** (u otra familia 7B similar) | Buen compromiso si se prioriza **latencia** o margen de VRAM |
| Más capacidad | Qwen2.5-Coder **14B** (u otra familia 14B similar en Ollama) | **Validado en el hardware del equipo:** corre bien; suele dar mejor calidad / tool-use que 7B a costa de tokens/s |
| Evitar en esta clase de PC | 70B+, modelos sin cuantizar muy grandes | Fuera de objetivo realista |

**Defecto recomendado para el producto:** **14B** como modelo **principal** local si ya está validado; **7B** como variante “rápida” (perfiles Explore/Guide o modo `--fast`) si implementáis dos nombres. La elección fina queda en **D1 / D11** + tag exacto en Ollama.

### 2.3 Implicaciones para diseño (importantes)

- Los **perfiles de agente** ([architecture-legacy-es.md §2.7](../archive/architecture-legacy-es.md), [agent-profiles.md](./agent-profiles.md)) permiten asignar un modelo **más pequeño** al explorador o al “guide” y reservar el modelo grande para el bucle principal; en Ollama son distintos `model` en la misma API.
- Los modelos locales suelen ir **por detrás** de los cloud en **seguimiento de formato JSON** para herramientas. Refuerza la decisión **D2**:
  - Probar **function / tool calling** nativo si el backend lo expone de forma usable.
  - Tener **plan B**: esquema de herramientas en texto + JSON estricto en el prompt y reintentos ante parseo fallido (el §8 ya pide límite de turnos para evitar bucles infinitos).

### 2.4 Límites honestos con RTX 4050 + 32 GB RAM

- **Sí:** desarrollo con IA local, automatización, código + herramientas acotadas, sesiones razonables con compactación ([ARCHITECTURE_LEGACY_ES.md §2.5](../archive/architecture-legacy-es.md), [CONTEXT_COMPACTION.md](./context-compaction.md)).
- **No o con expectativas bajas:** varios modelos gigantes en paralelo, “multi-agente” con muchas instancias, contextos enormes sin poda agresiva.

### 2.5 VRAM real (RTX 4050) y cuántos modelos conviene cargar

En **laptop con RTX 4050** lo habitual son **6 GB VRAM**. Eso condiciona todo lo demás:

- Un **7B** en cuantización tipo **Q4** suele ocupar del orden de **~4–5 GB** de VRAM; encaja holgado en GPU si la prioridad es **respuesta rápida**.
- Un **14B** en cuantización agresiva (p. ej. Q4/Q5 según build Ollama) puede **seguir siendo usable** en portátil 4050 cuando **parte del peso** va a **RAM** (32 GB) o cuando la **cuantización** y el runtime lo permiten — en muchos equipos 6 GB VRAM **sí** soportan 14B vía **offload** sin ser “imposible”; lo genérico es más **tokens/s** que “no arranca”. **En este proyecto el 14B coder está validado: corre bien**; seguid midiendo **tokens/s** y **tool** tras actualizar Ollama o el tag del modelo.
- **Dos modelos “grandes” a la vez** en la misma GPU **siguen sin ser realistas**; Ollama normalmente **descarga el anterior** al cargar otro → **latencia de cold start** al alternar 7B ↔ 14B. Útil definir **un default** (p. ej. 14B para el bucle principal) y cambiar a 7B solo cuando marquéis “modo rápido”.
- Con **32 GB RAM**, el offload del 14B es lo que hace viable el uso diario en hardware “apretado” de VRAM; si algún día la sesión se siente pesada, reducir **contexto** (§2.5 / compactación) pesa más que cambiar de familia.

**Conclusión:** **Principal local:** **14B coder** (validado). **Opcional:** **7B** para tareas cortas o perfiles baratos; **3B–4B** solo si más adelante justificáis **Explore/clasificador** (**D17**) y el swap de modelo compensa.

### 2.6 Economía por tipo de agente (referencia §2 de [agent-profiles.md](./agent-profiles.md))

La “economía” son tres palancas: **tamaño de modelo**, **adjuntos de contexto** (ya documentado: Explore/Plan sin reglas pesadas), y **`max_tokens`** / longitud de salida esperada.

| Perfil (ref.) | Objetivo | Modelo local (hardware validado) | Notas |
|-----------------|----------|----------------------------------|--------|
| **General-Purpose** | Comodín, edición, tools varias | **14B coder** (principal validado) o **7B** si se prioriza latencia | Default recomendado: **14B** para calidad/tool-use; **7B** como modo rápido. |
| **Explore** | Solo lectura, búsqueda en repo | **7B** o **14B** con `max_tokens` bajo | El ahorro grande sigue siendo **context policy** (sin reglas masivas), no solo bajar tamaño. |
| **Plan** | 3–5 archivos críticos | **14B** o **7B** + prompt acotado | Lo crítico es **contexto limpio**; 14B puede ayudar en planes difíciles si la latencia vale la pena. |
| **Verification** | PASS/FAIL + lectura | Igual que **Explore** o **7B** con plantilla de veredicto | Si corre en background, podéis tolerar modelo algo más lento. |
| **Guide** (docs) | Web + lectura, `dontAsk` | **3B–7B** según calidad de respuesta; priorizar **baja latencia** | Riesgo: `dontAsk` sin **D17** sólido es peligroso (véase [agent-profiles.md](./agent-profiles.md) §5). |
| **Status line** | Edit mínimo + Read | **7B** o modelo pequeño si la tarea es trivial | Bajo uso. |
| **Clasificador auto-modo** (**D17**) | XML corto, sí/no | **Modelo pequeño y rápido** (3B–4B) si implementáis vía Ollama | Idealmente **máximo tokens** muy bajo; importa latencia, no “sabiduría”. |

No es obligatorio mapear 1:1 “cada perfil = un GGUF distinto”. Con **14B validado**, muchos equipos usan **14B como único motor** + políticas de contexto ([agent-profiles.md](./agent-profiles.md) §3); **7B** añade valor sobre todo como **atajo de latencia** o segundo nombre cuando el swap lo toleráis.

### 2.7 Ollama frente a LM Studio (¿merece la pena otro motor?)

En **Windows + NVIDIA**, ambos suelen apoyarse en **llama.cpp**: el rendimiento bruto suele ser **muy parecido** (diferencias pequeñas según build y modelo). Diferencias útiles para **vuestro** diseño (cliente Go, HTTP, perfiles):

| Criterio | Ollama | LM Studio |
|----------|--------|-----------|
| **Integración headless / CI** | Muy natural (servicio, `ollama pull`, sin GUI) | Posible vía servidor local; más pensado en escritorio |
| **API estilo OpenAI** | `http://127.0.0.1:11434/v1` | `http://127.0.0.1:1234/v1` (puerto típico) |
| **Catálogo de modelos** | Registro curado + Modelfiles | Navegación Hugging Face / GGUF fino en GUI |
| **Dos servidores a la vez** | Sí (puertos distintos) | Sí, pero **dos procesos sirviendo LLM en la misma GPU** compiten por VRAM: en 4050 **no** es práctico “Ollama para el orquestador + LM Studio para otro agente” en paralelo con modelos medianos |
| **Licencia producto** | MIT, clara para embeber mentalmente en tooling | App propietaria; revisad términos si es comercial |

**Recomendación para el asistente Go:** mantened **Ollama como backend principal** (**D10**) y la **misma familia de API** para todo lo local: ya soporta **varios nombres de modelo** en el mismo servidor (cambio según perfil). **LM Studio** puede ayudar como **laboratorio**: probar quant/tamaño/m modelo que aún no está en el registro de Ollama, o afinar en GUI; si algo queda fijo, **importarlo a Ollama** o fijarlo vía `Modelfile` y seguir con **una sola URL** en `internal/llm` para no multiplicar código y operación.

Opcionalmente **D7** puede definir **dos URLs** (`OLLAMA_HOST` + `LLM_STUDIO_HOST`) solo si necesitáis un modelo **exclusivo** de un ecosistema; para MVP es **YAGNI**.

---

## 3. Herramientas de imagen y vídeo (manos separadas del LLM)

El **LLM** (Ollama, §2) no genera píxeles: imagen y vídeo son **procesos aparte**, invocados como **herramientas** (`internal/tools`) con HTTP/subproceso, permisos y colas. Panorama de modelos open-weights (2026): [The Best Open-Source Image Generation Models in 2026 — BentoML](https://www.bentoml.com/blog/a-guide-to-open-source-image-generation-models) (incluye FLUX.2, Stable Diffusion, Z-Image, Qwen-Image, GLM-Image, HunyuanImage, FAQ LoRA/ComfyUI/copyright).

### 3.1 Principio de integración en nuestro diseño

| Herramienta conceptual | Integración típica | Riesgo |
|------------------------|-------------------|--------|
| `generate_image` | Cliente HTTP hacia un **servidor de difusión** (A1111, ComfyUI+API, BentoML, u otro) o comando acotado | `network`, GPU, tiempo de generación |
| `generate_video_clip` | Mismo patrón: **API o workflow** (p. ej. Stable Video Diffusion, AnimateDiff en ComfyUI); casi siempre **más lento** y más VRAM que imagen | Cola obligatoria; **contención** con Ollama §3.5 |

**Reglas transversales:** políticas **Permissions** ([ARCHITECTURE_LEGACY_ES.md §2.3](../archive/architecture-legacy-es.md)) y encaje en el bucle §3, timeouts agresivos, y **no** bloquear el REPL: job en **background** + notificación o path a fichero cuando termine. **Copyright:** uso comercial de salidas y datasets de entrenamiento es zona gris; el propio artículo enlazado insiste en **riesgo legal** y “mantenerse informado”.

### 3.2 Familias de modelos (qué encaja con GPU **pequeña** vs “estado del arte”)

Resumen alineado con la guía enlazada; **requisitos VRAM** son orientativos (quant, resolución y runtime cambian mucho).

| Familia | Notas (síntesis) | Realismo **RTX 4050 ~6 GB** + 32 GB RAM |
|---------|------------------|----------------------------------------|
| **Stable Diffusion** (1.5, XL, 3.x, Turbo, Lightning…) | Ecosistema enorme; A1111/ComfyUI; [LoRA](https://www.bentoml.com/blog/a-guide-to-open-source-image-generation-models) para estilos | **Primera opción local:** SD **1.5** o variantes **rápidas** (p. ej. Lightning/Turbo) con resolución modrada; SDXL puede requerir optimizaciones “low VRAM”. |
| **Z-Image-Turbo** (~6B, Apache-2.0) | Rápido, buen texto bilingüe según el artículo; orientado a **≤16 GB VRAM** en versión consumer | **Probar** en vuestra máquina; puede ir justo en 6 GB o con offload a RAM (más lento). |
| **FLUX.2** (Black Forest Labs) | Varios tiers: **\[dev\]** ~32B open-weight; **\[klein\]** 9B/4B “edge” | El artículo cita **~13 GB VRAM** para la variante **4B** de Klein: **no** es el objetivo natural de una 4050 6 GB salvo otro equipo o **API** gestionada (**\[pro\]/\[flex\]** no son weights locales típicos). Revisar **licencia** comercial en checkpoints BFL. |
| **Qwen-Image** / **Qwen-Image-Lightning** | Texto en imagen y edición; Lightning acelera pasos | Tamaños grandes en checkpoints “full”; valorar solo si hay build **ligera** probada en 6 GB o servicio remoto. |
| **GLM-Image** | Fuerte en **tipografía** y layouts densos | Arquitectura ~9B+7B: **pesada** para este portátil como primera opción. |
| **HunyuanImage-3.0** | MoE muy grande (orden **80B** total en el artículo) | **No** local en este hardware. |

**Conclusión práctica:** para **implementar ya** en local con vuestra GPU, el camino más predecible es **Stable Diffusion vía stack maduro** (§3.4) + modelo acotado; añadir Z-Image u otros tras **medir VRAM/tiempo** reales.

### 3.3 Vídeo local

- En el mismo ecosistema **Stable Diffusion** suele citarse **Stable Video Diffusion** u otros pipelines (el artículo enlaza SD con vídeo/3D a alto nivel).
- **AnimateDiff** y similares suelen vivir en **ComfyUI** como grafos: más flexibles, más fricción operativa.
- **Esperativas:** vídeo = **más VRAM**, más tiempo y más fallos; tratadlo como **fase posterior** a imagen estable, con **cola única** y un solo “dueño” de GPU a la vez (§3.5).

### 3.4 Cómo servir el generador (para que `internal/tools` sea delgado)

| Opción | Ventaja | Inconveniente |
|--------|---------|----------------|
| **[AUTOMATIC1111](https://github.com/AUTOMATIC1111/stable-diffusion-webui)** | API relativamente simple, muchos tutoriales, inicio rápido | Menos flexible que grafos complejos; despliegue “producción” puede requerir cuidado |
| **[ComfyUI](https://github.com/comfyanonymous/ComfyUI)** | Pipelines avanzados, nodos; el artículo enlaza **comfy-pack** para empaquetar y exponer workflows como API | Curva de aprendizaje; reproducibilidad entre máquinas |
| **BentoML / contenedor propio** | Patrón “servir cualquier modelo” como en la guía | Más trabajo inicial; útil si ya estandarizáis inferencia |

**Contrato sugerido hacia Go:** un único `IMAGE_GEN_BASE_URL` (y opcional `VIDEO_GEN_BASE_URL`) en **D7**; `generate_image` hace `POST` con prompt + tamaño, recibe ruta o bytes; errores y timeouts claros para el orquestador.

### 3.5 Contención GPU: Ollama (LLM) + difusión

En la misma máquina **no conviene** ejecutar **14B en Ollama** y **una difusión pesada** al **mismo tiempo** en la misma GPU sin cuotas. Patrones:

1. **Serializar:** herramienta imagen que **espera** o devuelve “ocupado” si el LLM tiene turno (o viceversa).
2. **Modo explícito:** flag/config “solo imagen” que **para** o pausa llamadas al LLM (o usa otro host).
3. **Segundo dispositivo** o **API cloud** para imagen si el flujo debe ser siempre concurrente.

Documentad la política en **Permissions** + mensajes al usuario (“generación en curso, 30–120 s”).

### 3.6 Checklist implementación (imagen → vídeo)

1. Elegir stack (**A1111** o **ComfyUI**), modelo SD acorde a §3.2, verificar VRAM con **una** resolución fija.
2. Exponer **HTTP** estable (puerto en `.env.example`); probar con `curl` **sin** el asistente Go.
3. Implementar `generate_image` en Go: timeout, tamaño máximo de salida, path bajo directorio de trabajo o temp.
4. Añadir categoría de permiso **media/generación** (riesgo distinto de `read_file`).
5. Vídeo: clonar patrón con cola, timeouts mayores, y descripción de límites en este doc.

---

## 4. Memoria entre sesiones

- El diagrama tipo “goclaw” incluye **memoria**. En nuestra tabla §4.1 aún no hay paquete dedicado: puede ser **fase v2/v3** (ficheros `MEMORY.md`, SQLite, o MCP). Para **solo local**, empezar por **archivos en disco** bajo `.assistant/` evita servicios extra.

---

## 5. Mapa “goclaw simplificado” → paquetes Go

```
[Usuario] → internal/channel (REPL/TUI)
         → internal/orchestrator
              ↔ internal/llm     → HTTP → Ollama (Qwen Coder 7B/14B, …)
              ↔ internal/session
              → internal/permissions → internal/tools
                   ├── read_file, bash, web_search, web_fetch  (MVP)
                   ├── glob, grep  (v1 — regla §2.1 [ARCHITECTURE_LEGACY_ES.md](../archive/architecture-legacy-es.md))
                   ├── write, edit  (v2)
                   ├── image optional → A1111 API
                   └── video optional → proceso externo (fase tardía)
```

El **system prompt** debe reforzar [§2.1](../archive/architecture-legacy-es.md) (herramientas dedicadas); los modelos locales suelen beneficiarse **más** de esa regla explícita.

---

## 6. Checklist de integración (orden sugerido)

1. Instalar **Ollama**, descargar modelo(s) coder (**14B** validado en equipo; **7B** opcional para modo rápido), comprobar `curl` a la API local desde Go.
2. Implementar `internal/llm` con **una** URL base configurable y prueba de **una** conversación sin herramientas.
3. Añadir **una** herramienta trivial (`read_file`) y medir si el modelo obedece el formato acordado (D2); incluir en prompt la **regla de herramientas dedicadas** (D12).
4. Añadir `glob`/`grep` antes de alentar búsquedas vía `bash` (§2.1 del archivo principal).
5. Sólo después: `web_fetch` con política de red (§8.2 del archivo principal).
6. Imagen: tras MVP estable — §3.6; `.env.example` con `IMAGE_GEN_BASE_URL`; medir **contención GPU** §3.5 con Ollama.
7. Vídeo: después de imagen estable; cola y expectativas §3.3.

---

## 7. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: Ollama, modelos, límites hardware, herramientas media como tools opcionales, mapa a paquetes Go. |
| 2026-04-07 | Mapa y checklist alineados con **§2.1** herramientas dedicadas (glob/grep v1, write/edit v2, D12). |
| 2026-04-07 | §2.3: perfiles de agente y modelo barato vs principal (Ollama). |
| 2026-04-07 | §2.5–§2.7: VRAM 4050 (~6 GB), cuántos modelos, tabla perfil→modelo, Ollama vs LM Studio y una sola URL por defecto. |
| 2026-04-07 | **§1.1** panorama open-weights 2026 (privacidad, coste, especialización) filtrado por VRAM, coder+tools y §3 para imagen/vídeo. |
| 2026-04-07 | **§3** ampliado: panorama [BentoML imagen 2026](https://www.bentoml.com/blog/a-guide-to-open-source-image-generation-models), tabla VRAM vs familias, serving A1111/ComfyUI/BentoML, vídeo, contención GPU con Ollama, checklist §3.6. |
| 2026-04-07 | **14B coder validado** en hardware del equipo; §2.2–§2.6 y checklist priorizan 14B como principal local; 7B como variante rápida; §2.5 matiz offload/4050. |
