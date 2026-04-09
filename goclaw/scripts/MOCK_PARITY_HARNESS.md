# Mock Anthropic parity harness

Reproducible subset of integration tests that drive the real orchestrator and coordinator against [`testutil/mockserver`](../testutil/mockserver/) (Anthropic-compatible HTTP). No `ANTHROPIC_API_KEY` required.

## Artifacts

| Path | Role |
|------|------|
| [`mock_parity_scenarios.json`](mock_parity_scenarios.json) | Human-readable scenario ids (match `TestMockParityHarness` subtests) |
| [`run_mock_parity_harness.sh`](run_mock_parity_harness.sh) | Runs orchestrator + coordinator harness packages |
| [`internal/orchestrator/mock_parity_harness_test.go`](../internal/orchestrator/mock_parity_harness_test.go) | Bundles core orchestrator mock scenarios |
| [`internal/coordinator/mock_parity_harness_test.go`](../internal/coordinator/mock_parity_harness_test.go) | Bundles `spawn_agent` mock scenarios |

## Run

From the `goclaw` module root:

```bash
make parity
```

Or the shell wrapper:

```bash
./scripts/run_mock_parity_harness.sh
```

Pass extra `go test` flags after `--`:

```bash
./scripts/run_mock_parity_harness.sh -v
```

On Windows without bash, run the two commands from `mock_parity_scenarios.json` manually.

## Relation to claw-code

Inspired by the checklist style in [claw-code](https://github.com/ultraworkers/claw-code) `rust/MOCK_PARITY_HARNESS.md`; this repo uses Go `go test` entrypoints instead of a separate mock service binary.
