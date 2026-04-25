package tools

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type shellKind int

const (
	shellKindPOSIX shellKind = iota
	shellKindCMD
)

type shellSpec struct {
	Exec   string
	Prefix []string
	Kind   shellKind
}

func (s shellSpec) invocation(commandLine string) (string, []string) {
	args := append([]string{}, s.Prefix...)
	args = append(args, commandLine)
	return s.Exec, args
}

var (
	bashRuntimeGOOS       = runtime.GOOS
	bashLookPath          = exec.LookPath
	bashProbeShell        = probePOSIXShell
	newExecCommandContext = exec.CommandContext

	windowsShellCacheMu sync.Mutex
	windowsShellCached  bool
	windowsShellSpec    shellSpec
)

func activeShellSpec() shellSpec {
	return shellSpecForPlatform(bashRuntimeGOOS)
}

func shellSpecForPlatform(goos string) shellSpec {
	if goos != "windows" {
		return shellSpec{Exec: "/bin/bash", Prefix: []string{"-c"}, Kind: shellKindPOSIX}
	}
	return resolveWindowsShellSpec()
}

func resolveWindowsShellSpec() shellSpec {
	windowsShellCacheMu.Lock()
	defer windowsShellCacheMu.Unlock()
	if windowsShellCached {
		return windowsShellSpec
	}
	for _, name := range []string{"bash", "sh"} {
		path, err := bashLookPath(name)
		if err != nil {
			continue
		}
		if bashProbeShell(path) {
			windowsShellSpec = shellSpec{Exec: path, Prefix: []string{"-c"}, Kind: shellKindPOSIX}
			windowsShellCached = true
			return windowsShellSpec
		}
	}
	windowsShellSpec = shellSpec{Exec: "cmd.exe", Prefix: []string{"/C"}, Kind: shellKindCMD}
	windowsShellCached = true
	return windowsShellSpec
}

func probePOSIXShell(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	cmd := newExecCommandContext(ctx, path, "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		return false
	}
	return ctx.Err() == nil
}

func hasWorkingPOSIXShellOnWindows() bool {
	return shellSpecForPlatform("windows").Kind == shellKindPOSIX
}

func resetBashShellResolverForTests() {
	windowsShellCacheMu.Lock()
	defer windowsShellCacheMu.Unlock()
	bashRuntimeGOOS = runtime.GOOS
	bashLookPath = exec.LookPath
	bashProbeShell = probePOSIXShell
	newExecCommandContext = exec.CommandContext
	windowsShellCached = false
	windowsShellSpec = shellSpec{}
}
