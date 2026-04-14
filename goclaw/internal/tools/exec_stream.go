package tools

import (
	"bufio"
	"bytes"
	"io"
	"os/exec"
	"sync"
)

// runCommandWithProgressPipes starts c with separate stdout/stderr pipes, forwards each
// completed line to reporter (orchestrator adapters map this to the active tool name),
// and returns captured output with the same cap and truncation suffix as CombinedOutput paths.
func runCommandWithProgressPipes(c *exec.Cmd, reporter ProgressReporter) ([]byte, error) {
	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}

	var (
		mu  sync.Mutex
		buf bytes.Buffer
	)
	drain := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, MaxBashOutput), MaxBashOutput)
		for sc.Scan() {
			line := sc.Text()
			mu.Lock()
			if buf.Len() < MaxBashOutput {
				buf.WriteString(line)
				buf.WriteByte('\n')
			}
			mu.Unlock()
			reporter.OnProgress("", line)
		}
		if err := sc.Err(); err != nil {
			mu.Lock()
			buf.WriteString("\n[scanner error: " + err.Error() + "]")
			mu.Unlock()
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); drain(stdoutPipe) }()
	go func() { defer wg.Done(); drain(stderrPipe) }()
	wg.Wait()

	waitErr := c.Wait()

	mu.Lock()
	out := buf.Bytes()
	mu.Unlock()

	if len(out) > MaxBashOutput {
		out = out[:MaxBashOutput]
		out = append(out, []byte("\n[output truncated]")...)
	}
	return out, waitErr
}
