package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
)

type doctorCheckSummary struct {
	lines    []string
	ollamaOK bool
}

func appendDoctorSection(lines []string, section []string) []string {
	if len(section) == 0 {
		return lines
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	return append(lines, section...)
}

func doctorIntroLines(rt *ChatRuntime) []string {
	cfg := rt.Cfg
	taskModelsLine := "task_models: (none)"
	if n := len(cfg.TaskModels); n > 0 {
		taskModelsLine = fmt.Sprintf("task_models: %d role(s) mapped", n)
	}
	lines := []string{
		"goclaw doctor",
		"",
		fmt.Sprintf("tool path root (default project): %s", rt.Workdir),
	}
	if ld := strings.TrimSpace(rt.LaunchDir); ld != "" {
		if wd := strings.TrimSpace(rt.Workdir); filepath.Clean(ld) != filepath.Clean(wd) {
			lines = append(lines, fmt.Sprintf("launch cwd: %s (project .goclaw loads from here)", ld))
		}
	}
	lines = append(lines,
		fmt.Sprintf("session:   %s", rt.Sess.ID),
		fmt.Sprintf("provider:  %s", cfg.Provider),
		fmt.Sprintf("model:     %s", cfg.Model()),
		fmt.Sprintf("ollama_require_wire_tools: %v", cfg.OllamaRequireWireTools),
		fmt.Sprintf("tui_chat_max_iterations (effective): %d", cfg.EffectiveTUIChatMaxIterations()),
		fmt.Sprintf("task_model_router: %s", config.NormalizeTaskModelRouter(cfg.TaskModelRouter)),
		taskModelsLine,
		fmt.Sprintf("auto_profile_intent: %s", config.NormalizeAutoProfileIntent(cfg.AutoProfileIntent)),
		fmt.Sprintf("auto_direct_coding_profile: %s", config.NormalizeAutoDirectCodingProfile(cfg.AutoDirectCodingProfile)),
		fmt.Sprintf("action_repair_escalation: %v", cfg.ActionRepairEscalation),
		fmt.Sprintf("profile:   %s", agents.DisplayProfileName(rt.Profile.Name)),
		fmt.Sprintf("runtime class: %s", agents.RuntimeClassName(rt.Profile.Name, rt.Profile.ReadOnly)),
		fmt.Sprintf("real tool calling required now: %v", agents.IsBuildLiteProfile(rt.Profile)),
		fmt.Sprintf("file tools: %s", doctorFileToolsLine(rt.Profile)),
		fmt.Sprintf("tools:     %s", enabledDisabled(!rt.DisableTools)),
	)
	return lines
}

func doctorWebConfigLines(cfg config.Config) []string {
	webBackend, webBackendOK := config.NormalizeWebSearchBackend(cfg.WebSearchBackend)
	var lines []string
	if !webBackendOK && strings.TrimSpace(cfg.WebSearchBackend) != "" {
		lines = append(lines, fmt.Sprintf("web_search: unknown backend %q (using ddg)", strings.TrimSpace(cfg.WebSearchBackend)))
	} else {
		lines = append(lines, fmt.Sprintf("web_search backend: %s", webBackend))
	}
	if webBackend == "brave" {
		lines = append(lines, checkLine("brave search api key configured", strings.TrimSpace(cfg.BraveSearchAPIKey) != ""))
	}
	if webBackend == "serpapi" {
		lines = append(lines, checkLine("serpapi key configured", strings.TrimSpace(cfg.SerpAPIKey) != ""))
	}
	if webBackend != "ddg" {
		lines = append(lines, fmt.Sprintf("  - fallback to duckduckgo: %v", cfg.WebSearchFallbackDDG))
	}
	return lines
}

func doctorLocalStateLines(rt *ChatRuntime) []string {
	cfg := rt.Cfg
	return []string{
		fmt.Sprintf("user config dir: %s", cfg.UserConfigDir),
		fmt.Sprintf("sessions dir:    %s", filepath.Join(cfg.UserConfigDir, "sessions")),
		fmt.Sprintf("memory dir:      %s", filepath.Join(cfg.UserConfigDir, "memory")),
	}
}

func doctorOptionalIntegrationLines(rt *ChatRuntime) []string {
	var lines []string
	lines = append(lines, ideBridgeDoctorLines(rt)...)
	lines = append(lines, "")
	lines = append(lines, telegramDoctorLines(rt)...)
	return lines
}

func doctorCheckLines(rt *ChatRuntime) doctorCheckSummary {
	cfg := rt.Cfg
	lines := []string{"checks:"}
	var ollamaOK bool
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "anthropic":
		lines = append(lines, "  x provider \"anthropic\" is no longer supported - set \"provider\" to \"ollama\"")
	case "openai_compatible":
		lines = append(lines, "  x provider \"openai_compatible\" is not supported - goclaw uses local Ollama only; set \"provider\" to \"ollama\" and use ollama_model")
	default:
		ollamaHost := effectiveOllamaHost(cfg.OllamaHost)
		probe := rt.OllamaProbe
		ollamaOK = probe.Reachable
		lines = append(lines, checkLine("ollama host reachable", ollamaOK))
		lines = append(lines, fmt.Sprintf("  - ollama host: %s", ollamaHost))
		lines = append(lines, fmt.Sprintf("  - ollama_num_ctx: %d", cfg.OllamaNumCtx))
		if !rt.DisableTools && cfg.OllamaNumCtx > 0 && cfg.OllamaNumCtx < 8192 {
			lines = append(lines, "  ! ollama_num_ctx below 8192 may be tight for tool schemas - raise toward 16384+ if the model rejects tools or truncates system context")
		}
		if ollamaOK {
			modelName := strings.TrimSpace(cfg.Model())
			if modelName != "" {
				lines = append(lines, checkLine("configured model in local ollama library", probe.ModelInLibrary))
				if !probe.ModelInLibrary {
					lines = append(lines, "    fix: ollama pull "+modelName)
				}
			}
			lines = append(lines, ollamaExtraModelCheckLines(cfg, probe.ModelNames)...)
		}
		if OllamaFunctionToolsDropped(rt) {
			lines = append(lines, "  x ollama rejected tool calling - running text-only this session")
		} else {
			lines = append(lines, "  ok ollama tool calling active")
		}
		if agents.IsBuildLiteProfile(rt.Profile) {
			lines = append(lines, "  - build-lite runtime bypasses normalize_input_language, task_model_router, llm_compaction, memory injection, skills injection, and full project context on the main turn path")
		}
	}
	lines = append(lines, checkLine("session store initialized", rt.Store != nil))
	lines = append(lines, mcpSummaryLines(rt)...)
	return doctorCheckSummary{lines: lines, ollamaOK: ollamaOK}
}

func doctorHintLines(rt *ChatRuntime, ollamaOK bool) []string {
	hints := hintLines(rt.Cfg, ollamaOK, rt.OllamaProbe.ModelInLibrary, rt.DisableTools, OllamaFunctionToolsDropped(rt))
	hints = append(hints, profileHintLines(rt.Profile)...)
	hints = append(hints, writeToolApprovalHintLines(rt)...)
	hints = append(hints, mcpConnectionHintLines(rt.Cfg, rt)...)
	if len(hints) == 0 {
		return nil
	}
	return append([]string{"hints:"}, hints...)
}

func DoctorReportFromRuntime(_ context.Context, rt *ChatRuntime) string {
	if rt == nil {
		return "doctor: no runtime"
	}
	checks := doctorCheckLines(rt)
	var lines []string
	lines = appendDoctorSection(lines, doctorIntroLines(rt))
	lines = appendDoctorSection(lines, doctorWebConfigLines(rt.Cfg))
	lines = appendDoctorSection(lines, doctorLocalStateLines(rt))
	lines = appendDoctorSection(lines, doctorOptionalIntegrationLines(rt))
	lines = appendDoctorSection(lines, checks.lines)
	lines = appendDoctorSection(lines, doctorHintLines(rt, checks.ollamaOK))
	lines = appendDoctorSection(lines, mcpServerSection(rt))
	lines = appendDoctorSection(lines, toolPermissionSection(rt))
	lines = appendDoctorSection(lines, pluginSkillMemorySection(rt))
	return strings.Join(lines, "\n")
}
