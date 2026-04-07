# Go vs Rust para un asistente CLI / agente (resumen)

Documento breve para comparar lenguajes en el tipo de producto descrito en [ARCHITECTURE.md](ARCHITECTURE.md) (orquestador, llamadas HTTP al LLM, herramientas, permisos). **No es un benchmark propio:** sintetiza criterios habituales y enlaces útiles.

---

## 1. Qué domina el coste en un “asistente” típico

- **Latencia de red** hacia el proveedor del modelo (o Ollama local) suele mandar más que el runtime del binario.
- El proceso pasa por **muchos I/O**: HTTP, lectura de archivos, `exec` de subprocesos, posibles streams.
- Por tanto, la elección de lenguaje raramente se decide solo por “micro-FLOPs”; importan **velocidad de desarrollo**, **modelo de concurrencia**, **seguridad operativa** y **tamaño/complexidad del binario**.

---

## 2. Go — ventajas típicas para este caso

| Aspecto | Por qué ayuda al asistente |
|--------|----------------------------|
| **Concurrencia** | Goroutines + `context` encajan bien en bucles “varias herramientas / timeouts / cancelación” sin arrastrar async colorado en todo el tipo. |
| **Red y CLI** | Cliente HTTP en stdlib, integración habitual para CLIs y servicios; muchas guías alinean Go con herramientas **network-heavy** ([comparativa honesta Rust vs Go para CLI](https://unixy.io/blog/rust-vs-go-cli-tools/)). |
| **Iteración** | Compilación y onboarding del equipo suele ser **más rápido** que en Rust para proyectos medianos ([misma fuente](https://unixy.io/blog/rust-vs-go-cli-tools/)). |
| **Binario único** | Despliegue simple; tamaño suele ser razonable (a veces mayor que Rust mínimo, pero irrelevante frente al peso de modelos). |
| **Ecosistema agente** | Hay implementaciones reales de agentes ligeros en Go (p. ej. en comparativas del estilo “Claw family” se menciona [PicoClaw en Go](https://heyferrante.com/ai-agent-frameworks-february-2026) como apuesta por huella y un solo binario; tratar como **dato cualitativo del ecosistema**, no como veredicto absoluto). |

**Desventajas / matices**

- Sin **borrow checker**: correctness en memoria depende de disciplina y pruebas; para parsing agresivo o formatos binarios críticos, Rust aporta más garantías estáticas.
- Rendimiento **CPU puro** peor que Rust optimizado; pocas partes del asistente suelen ser el cuello de botella.

---

## 3. Rust — ventajas típicas para este caso

| Aspecto | Por qué ayuda al asistente |
|--------|----------------------------|
| **Seguridad de memoria** | Ausencia de data races en compiled-safe code; interesa en bins que parsean entrada no confiable o implementan sandboxes/cajas de arena. |
| **Binarios pequeños** | Con optimización y poco runtime, se pueden lograr CLIs muy compactos; autores de herramientas tipo agente argumentan tamaños agresivos (p. ej. narrativas en torno a [ZeroClaw](https://www.lushbinary.com/blog/zeroclaw-openclaw-personal-ai-agents-compared-2026/) — contrastar siempre con mediciones propias). |
| **CLI madura** | `clap` y patrones de errores con `Result` están muy asentados ([unixy.io](https://unixy.io/blog/rust-vs-go-cli-tools/)). |
| **Ecosistema “agent CLI”** | En 2025–2026 proliferan runtimes y herramientas en Rust orientadas a agentes (p. ej. artículos de comparación [MicroClaw vs ZeroClaw vs Moltis](https://medium.com/@everettjf/rust-agent-runtime-showdown-microclaw-vs-zeroclaw-vs-moltis-df1ecb85c676)); refleja **momentum** más que obligación de usar Rust. |

**Desventajas / matices**

- **Curva de aprendizaje** más alta y tiempos de compilación mayores en proyectos async grandes ([unixy.io](https://unixy.io/blog/rust-vs-go-cli-tools/)).
- **Async (Tokio)** añade complejidad frente a goroutines para equipos nuevos en Rust.
- Benchmarks HTTP cliente-vs-cliente **dependen mucho** de reutilización de conexiones y configuración; hay hilos donde Go gana y otros donde Rust igual tras tunear ([issue de reqwest vs Go](https://github.com/seanmonstar/reqwest/issues/1815), [comparación concurrente](https://github.com/seanmonstar/reqwest/issues/2457)). No sacar conclusiones globales de un solo número.

---

## 4. Veredicto práctico (para *tu* arquitectura)

| Si priorizas… | Suele favorecer a… |
|---------------|-------------------|
| Entrega rápida, equipo centrado en Go, mucho I/O y orquestación | **Go** (ya alineado con [ARCHITECTURE.md](ARCHITECTURE.md) D0) |
| Garantías máximas en componentes críticos, bins mínimos, política de memoria estricta | **Rust** (o un **híbrido**: núcleo Rust + scripting, más complejo operativamente) |
| Paridad con el ecosistema actual de “agent runtimes” en Rust | Rust tiene más **proyectos ejemplo** recientes; Go tiene menos pero suficiente para CLI completo |

**Conclusión corta:** para un asistente con bucle LLM + herramientas + permisos, **Go y Rust son ambos viables**. La diferencia suele ser **coste de equipo y tiempo**, no “imposible en uno”. Vuestra decisión explícita de **Go** sigue siendo coherente con el perfil I/O del producto; Rust sería replanteo estratégico, no corrección técnica obligada.

---

## 5. Lecturas enlazadas

- [Rust vs Go for CLI Tools: The Honest Comparison — unixy.io](https://unixy.io/blog/rust-vs-go-cli-tools/)
- [AI Agent Frameworks Compared (incl. Go y Rust) — Hey Ferrante, feb 2026](https://heyferrante.com/ai-agent-frameworks-february-2026)
- [Comparativa de agentes personales (varios lenguajes)](https://www.lushbinary.com/blog/zeroclaw-openclaw-personal-ai-agents-compared-2026/)
- [reqwest issue: rendimiento cliente HTTP vs Go (matices de configuración)](https://github.com/seanmonstar/reqwest/issues/1815)

---

## 6. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Primera versión: resumen con fuentes web, orientado a asistente CLI/agente. |
