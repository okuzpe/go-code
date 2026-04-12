# Local models and media — Go architecture fit

Depth document linked to [CLAUDE.md](../../goclaw/CLAUDE.md) (**§2 ter**). Summarizes a **realistic 2025–2026 strategy** for a PC with **RTX 4050 + 32 GB RAM** and how it fits our design (CLI/orchestrator in **Go**, not Python). Current hub: [architecture.md](../architecture.md); GoClaw behavior: [goclaw/CLAUDE.md](../../goclaw/CLAUDE.md).

---

## 1. Verdict

- Building a "**Claude Code lite**"-style assistant with a **local LLM** is **coherent** with the §3 map: the `LLM_API` block becomes a **service on the same machine** (typically **Ollama** listening on `127.0.0.1`), and the Go client speaks **HTTP** just like with a remote provider.
- What external summaries call a "modern CLI stack" (**Typer + Rich + Prompt Toolkit**) is **Python**; in our project the "visible face" remains **Go** (`internal/channel`: simple REPL or TUI with a Go library if chosen in D8). No need to sacrifice terminal richness: equivalents exist in the Go ecosystem.

### 1.1 Open-weights landscape (2026) — what to leverage and what to filter

Internet summaries agree on the broad picture: **open-weights** models are gaining ground over closed APIs for **privacy**, long-term **cost**, **avoiding lock-in**, and **auditability**; there is **specialization** (general chat vs **coder** vs long **reasoning** vs image/video). Tools like **Ollama**, LM Studio, or Jan fit the "local execution" pattern.

**For our design the filter is not the market's maximum list, but:**

1. **Hardware:** rankings that cite DeepSeek-V3, "Llama 4", large Qwen, etc. typically assume **multiple GPUs** or **API** usage. With **RTX 4050 ~6 GB VRAM** (§2.5), the useful subset is **small/medium quantized variants** that exist in the [Ollama library](https://ollama.com/library) or equivalent — not the most-tweeted checkpoint.
2. **Product task:** priority is **coding + tool-use** (§2.2, **D2**), not just creative chat; use **"coder"** families and real **JSON / tools** tests, not just conversational benchmarks.
3. **Multimodal / image generation:** models like **Flux** or video pipelines go to section **§3** (optional tools), they do not replace the orchestrator LLM.
4. **Operational complexity:** the limits cited in summaries (deployment vs API) are real; that is why this doc fixes **Ollama as an external process** and **D7** for config, instead of embedding inference in the Go binary.

Concrete names change every month; **D11** should settle on **one** tested Ollama identifier on your PC, not a theoretical collection.

---

## 2. Local LLM (the brain)

### 2.1 Recommended runtime: Ollama

- **Why:** wide community adoption, simple installation, **GPU** use when suitable CUDA drivers are present, stable HTTP API for integration from Go without embedding `llama.cpp` in the binary.
- **Role in the architecture:** **external** process to the Go assistant; `internal/llm` implements a client pointing to `OLLAMA_HOST` (or configurable in D7).

### 2.2 Recent models for local use (coding / assistant)

| Profile | Model (indicative) | Note |
|---------|--------------------|------|
| Fast, lower VRAM | Qwen2.5-Coder **7B** (or similar 7B family) | Good trade-off if **latency** or VRAM headroom is the priority |
| Higher capability | Qwen2.5-Coder **14B** (or similar 14B family on Ollama) | **Validated on team hardware:** runs well; usually better quality / tool-use than 7B at the cost of tokens/s |
| Avoid on this class of PC | 70B+, large unquantized models | Outside the realistic target |

**Recommended default for the product:** **14B** as the **main** local model if already validated; **7B** as the "fast" variant (Explore/Guide profiles or `--fast` mode) if you implement two names. The fine choice stays in **D1 / D11** + exact Ollama tag.

### 2.3 Design implications (important)

- **Agent profiles** ([agent-profiles.md](./agent-profiles.md)) allow assigning a **smaller model** to the explorer or "guide" and reserving the large model for the main loop; in Ollama these are different `model` values on the same API.
- Local models usually **lag behind** cloud ones in **JSON format adherence** for tools. This reinforces decision **D2**:
  - Test native **function / tool calling** if the backend exposes it usably.
  - Have a **plan B**: tool schema in text + strict JSON in the prompt and retries on failed parsing (§8 already requests a turn limit to avoid infinite loops).

### 2.4 Honest limits with RTX 4050 + 32 GB RAM

- **Yes:** AI-assisted development, automation, code + bounded tools, reasonable sessions with compaction ([context-compaction.md](./context-compaction.md)).
- **No or with low expectations:** several giant models in parallel, "multi-agent" with many instances, huge contexts without aggressive pruning.

### 2.5 Real VRAM (RTX 4050) and how many models to load

On a **laptop with RTX 4050** the typical figure is **6 GB VRAM**. That shapes everything else:

- A **7B** in **Q4** quantization typically occupies around **~4–5 GB** of VRAM; fits comfortably on the GPU if **fast response** is the priority.
- A **14B** in aggressive quantization (e.g. Q4/Q5 depending on the Ollama build) can **still be usable** on a 4050 laptop when **part of the weight** goes to **RAM** (32 GB) or when the **quantization** and runtime allow it — on many machines 6 GB VRAM **does** support 14B via **offload** without being "impossible"; the practical result is more **tokens/s** than "won't start". **In this project the 14B coder is validated: it runs well**; keep measuring **tokens/s** and **tool** behavior after updating Ollama or the model tag.
- **Two "large" models at once** on the same GPU **remain unrealistic**; Ollama typically **unloads the previous one** when loading another → **cold start latency** when alternating between 7B ↔ 14B. Useful to define **one default** (e.g. 14B for the main loop) and switch to 7B only when you mark "fast mode".
- With **32 GB RAM**, 14B offload is what makes daily use viable on VRAM-constrained hardware; if a session ever feels sluggish, reducing **context** (§2.5 / compaction) matters more than switching families.

**Conclusion:** **Main local:** **14B coder** (validated). **Optional:** **7B** for short tasks or lightweight profiles; **3B–4B** only if you later justify **Explore/classifier** (**D17**) and the model swap is worth it.

### 2.6 Economy by agent type (reference §2 of [agent-profiles.md](./agent-profiles.md))

The "economy" has three levers: **model size**, **context attachments** (already documented: Explore/Plan without heavy rules), and **`max_tokens`** / expected output length.

| Profile (ref.) | Goal | Local model (validated hardware) | Notes |
|----------------|------|----------------------------------|-------|
| **General-Purpose** | Swiss army knife, editing, various tools | **14B coder** (validated main) or **7B** if latency is a priority | Recommended default: **14B** for quality/tool-use; **7B** as fast mode. |
| **Explore** | Read-only, repo search | **7B** or **14B** with low `max_tokens` | The big saving is still **context policy** (no heavy rules), not just model size reduction. |
| **Plan** | 3–5 critical files | **14B** or **7B** + bounded prompt | Critical thing is **clean context**; 14B can help on hard plans if latency is worth it. |
| **Verification** | PASS/FAIL + read | Same as **Explore** or **7B** with verdict template | If running in background, you can tolerate a slightly slower model. |
| **Guide** (docs) | Web + read, `dontAsk` | **3B–7B** depending on response quality; prioritize **low latency** | Risk: `dontAsk` without solid **D17** is dangerous (see [agent-profiles.md](./agent-profiles.md) §5). |
| **Status line** | Minimal Edit + Read | **7B** or small model if the task is trivial | Low usage. |
| **Auto-mode classifier** (**D17**) | Short XML, yes/no | **Small and fast model** (3B–4B) if implementing via Ollama | Ideally very low **max tokens**; latency matters, not "wisdom". |

It is not mandatory to map 1:1 "each profile = a different GGUF". With **14B validated**, many teams use **14B as the sole engine** + context policies ([agent-profiles.md](./agent-profiles.md) §3); **7B** adds value mainly as a **latency shortcut** or second name when the swap cost is acceptable.

### 2.7 Ollama vs LM Studio (is another engine worth it?)

On **Windows + NVIDIA**, both typically rely on **llama.cpp**: raw performance is usually **very similar** (small differences depending on build and model). Useful differences for **your** design (Go client, HTTP, profiles):

| Criterion | Ollama | LM Studio |
|-----------|--------|-----------|
| **Headless / CI integration** | Very natural (service, `ollama pull`, no GUI) | Possible via local server; more desktop-oriented |
| **OpenAI-style API** | `http://127.0.0.1:11434/v1` | `http://127.0.0.1:1234/v1` (typical port) |
| **Model catalog** | Curated registry + Modelfiles | Hugging Face / fine GGUF browsing in GUI |
| **Two servers at once** | Yes (different ports) | Yes, but **two LLM-serving processes on the same GPU** compete for VRAM: on a 4050 running both in parallel with medium models is **not practical** |
| **Product license** | MIT, clear for tooling | Proprietary app; review terms if commercial |

**Recommendation for the Go assistant:** keep **Ollama as the main backend** (**D10**) and the **same API family** for everything local: it already supports **multiple model names** on the same server (switch based on profile). **LM Studio** can help as a **laboratory**: test quant/size/model not yet in the Ollama registry, or tune in GUI; if something sticks, **import it to Ollama** or fix it via `Modelfile` and continue with **one URL** in `internal/llm` to avoid multiplying code and operations.

Optionally **D7** can define **two URLs** (`OLLAMA_HOST` + `LLM_STUDIO_HOST`) only if you need a model **exclusive** to one ecosystem; for most deployments this is **YAGNI** (one base URL is enough).

---

## 3. Image and video tools (separate from the LLM)

The **LLM** (Ollama, §2) does not generate pixels: image and video are **separate processes**, invoked as **tools** (`internal/tools`) via HTTP/subprocess, with permissions and queues. Open-weights landscape (2026): [The Best Open-Source Image Generation Models in 2026 — BentoML](https://www.bentoml.com/blog/a-guide-to-open-source-image-generation-models) (includes FLUX.2, Stable Diffusion, Z-Image, Qwen-Image, GLM-Image, HunyuanImage, LoRA/ComfyUI/copyright FAQ).

### 3.1 Integration principle in our design

| Conceptual tool | Typical integration | Risk |
|-----------------|---------------------|------|
| `generate_image` | HTTP client toward a **diffusion server** (A1111, ComfyUI+API, BentoML, or other) or bounded command | `network`, GPU, generation time |
| `generate_video_clip` | Same pattern: **API or workflow** (e.g. Stable Video Diffusion, AnimateDiff on ComfyUI); almost always **slower** and more VRAM than image | Mandatory queue; **contention** with Ollama §3.5 |

**Cross-cutting rules:** **Permissions** policies ([CLAUDE.md](../../goclaw/CLAUDE.md) (permissions)) and fit in the §3 loop, aggressive timeouts, and **no** blocking the REPL: job in **background** + notification or file path when done. **Copyright:** commercial use of outputs and training datasets is a gray area; the linked article emphasizes **legal risk** and "staying informed".

### 3.2 Model families (what fits small GPU vs state of the art)

Summary aligned with the linked guide; **VRAM requirements** are indicative (quant, resolution, and runtime vary significantly).

| Family | Notes (synthesis) | Realistic for **RTX 4050 ~6 GB** + 32 GB RAM |
|--------|-------------------|----------------------------------------------|
| **Stable Diffusion** (1.5, XL, 3.x, Turbo, Lightning…) | Huge ecosystem; A1111/ComfyUI; [LoRA](https://www.bentoml.com/blog/a-guide-to-open-source-image-generation-models) for styles | **First local option:** SD **1.5** or **fast variants** (e.g. Lightning/Turbo) at moderate resolution; SDXL may need low-VRAM optimizations. |
| **Z-Image-Turbo** (~6B, Apache-2.0) | Fast, good bilingual text per the article; oriented toward **≤16 GB VRAM** in consumer version | **Test** on your machine; may fit in 6 GB or with RAM offload (slower). |
| **FLUX.2** (Black Forest Labs) | Several tiers: **[dev]** ~32B open-weight; **[klein]** 9B/4B "edge" | The article cites **~13 GB VRAM** for the **4B Klein** variant: **not** the natural target for a 4050 6 GB except on another machine or **managed API** (**[pro]/[flex]** are not typical local weights). Review **commercial license** on BFL checkpoints. |
| **Qwen-Image** / **Qwen-Image-Lightning** | Text in image and editing; Lightning speeds up steps | Large sizes in "full" checkpoints; consider only if there is a tested **lightweight build** for 6 GB or a remote service. |
| **GLM-Image** | Strong in **typography** and dense layouts | ~9B+7B architecture: **heavy** for this laptop as a first choice. |
| **HunyuanImage-3.0** | Very large MoE (~**80B** total per the article) | **Not** local on this hardware. |

**Practical conclusion:** to **implement now** locally with your GPU, the most predictable path is **Stable Diffusion via the mature stack** (§3.4) + bounded model; add Z-Image or others after **measuring real VRAM/time**.

### 3.3 Local video

- In the same **Stable Diffusion** ecosystem **Stable Video Diffusion** or other pipelines are often mentioned (the article links SD with video/3D at a high level).
- **AnimateDiff** and similar typically live in **ComfyUI** as graphs: more flexible, more operational friction.
- **Expectations:** video = **more VRAM**, more time and more failures; treat it as a **phase after stable image**, with a **single queue** and one sole "GPU owner" at a time (§3.5).

### 3.4 How to serve the generator (to keep `internal/tools` thin)

| Option | Advantage | Disadvantage |
|--------|-----------|--------------|
| **[AUTOMATIC1111](https://github.com/AUTOMATIC1111/stable-diffusion-webui)** | Relatively simple API, many tutorials, quick start | Less flexible than complex graphs; "production" deployment may require care |
| **[ComfyUI](https://github.com/comfyanonymous/ComfyUI)** | Advanced pipelines, nodes; the article links **comfy-pack** for packaging workflows as API | Learning curve; reproducibility across machines |
| **BentoML / own container** | "Serve any model" pattern as in the guide | More initial work; useful if you already standardize inference |

**Suggested contract toward Go:** a single `IMAGE_GEN_BASE_URL` (and optional `VIDEO_GEN_BASE_URL`) in **D7**; `generate_image` does `POST` with prompt + size, receives path or bytes; clear errors and timeouts for the orchestrator.

### 3.5 GPU contention: Ollama (LLM) + diffusion

On the same machine it is **not advisable** to run **14B in Ollama** and a **heavy diffusion** at the **same time** on the same GPU without quotas. Patterns:

1. **Serialize:** image tool that **waits** or returns "busy" if the LLM has a turn (or vice versa).
2. **Explicit mode:** flag/config "image only" that **stops** or pauses LLM calls (or uses another host).
3. **Second device** or **cloud API** for image if the flow must always be concurrent.

Document the policy in **Permissions** + user messages ("generation in progress, 30–120 s").

### 3.6 Implementation checklist (image → video)

1. Choose stack (**A1111** or **ComfyUI**), SD model appropriate for §3.2, verify VRAM with **one** fixed resolution.
2. Expose **stable HTTP** (port in `.env.example`); test with `curl` **without** the Go assistant.
3. Implement `generate_image` in Go: timeout, maximum output size, path under work directory or temp.
4. Add **media/generation** permission category (different risk from `read_file`).
5. Video: clone the pattern with a queue, larger timeouts, and limits description in this doc.

---

## 4. Cross-session memory

- **goclaw:** on-disk memory under `~/.goclaw/memory/` + `MEMORY.md`, REPL `/memory`, optional auto-capture — see **D13** in [`CLAUDE.md`](../../goclaw/CLAUDE.md) and [memory-system.md](./memory-system.md). The previous paragraph described an **obsolete** design state; today the package exists in `internal/memory`.

---

## 5. Simplified goclaw map → Go packages

```
[User] → internal/channel (REPL/TUI)
       → internal/orchestrator
            ↔ internal/llm     → HTTP → Ollama (Qwen Coder 7B/14B, …)
            ↔ internal/session
            → internal/permissions → internal/tools
                 ├── read_file, glob, grep, bash, web_search, web_fetch, write_file, edit_file, patch, todo_write  (**shipped**)
                 ├── coordinator: spawn_agent, stop_task  (**shipped** profile `coordinator`)
                 ├── script (optional `allow_script`), MCP `mcp__*`, plugins/skills  (**shipped** based on config)
                 ├── image/video  (**no** built-in tools; §3 describes optional external integration)
                 └── generate_image / video  (**not implemented** in the binary; §3 pattern if added)
```

The **system prompt** must reinforce dedicated tools before bash ([CLAUDE.md](../../goclaw/CLAUDE.md) D12); local models usually benefit **more** from that explicit rule.

---

## 6. Integration checklist (suggested order)

1. Install **Ollama**, download coder model(s) (**14B** validated on team hardware; **7B** optional for fast mode), verify `curl` to the local API from Go.
2. Implement `internal/llm` with **one** configurable base URL and test with **one** tool-free conversation.
3. Add **one** trivial tool (`read_file`) and measure whether the model obeys the agreed format (D2); include the **dedicated tools rule** in the prompt (D12).
4. Add `glob`/`grep` before encouraging searches via `bash` (§2.1 of the main file).
5. Only then: `web_fetch` with network policy (§8.2 of the main file).
6. Image: when local chat is stable — §3.6; `.env.example` with `IMAGE_GEN_BASE_URL`; measure **GPU contention** §3.5 with Ollama (**tool not included** in goclaw by default).
7. Video: after stable image; queue and expectations §3.3.

---

## 7. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: Ollama, models, hardware limits, media tools as optional tools, Go package map. |
| 2026-04-07 | Map and checklist aligned with **§2.1** dedicated tools (glob/grep, write/edit, D12). |
| 2026-04-07 | §2.3: agent profiles and cheap model vs main (Ollama). |
| 2026-04-07 | §2.5–§2.7: VRAM 4050 (~6 GB), how many models, profile→model table, Ollama vs LM Studio and single default URL. |
| 2026-04-07 | **§1.1** open-weights landscape 2026 (privacy, cost, specialization) filtered by VRAM, coder+tools and §3 for image/video. |
| 2026-04-07 | **§3** expanded: [BentoML image landscape 2026](https://www.bentoml.com/blog/a-guide-to-open-source-image-generation-models), VRAM vs families table, serving A1111/ComfyUI/BentoML, video, GPU contention with Ollama, §3.6 checklist. |
| 2026-04-07 | **14B coder validated** on team hardware; §2.2–§2.6 and checklist prioritize 14B as main local; 7B as fast variant; §2.5 nuance on offload/4050. |
| 2026-04-12 | Translated from Spanish to English. |
