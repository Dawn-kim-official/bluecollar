# Putting this loop on Terminal-Bench

Terminal-Bench builds a container per task, hands an agent one instruction, and
runs its own test suite against what the agent left behind. The agent here is
`cmd/bluecollar`: the loop stays on the host and reaches the task through
`docker exec`, so the container is exactly the environment the benchmark made.

## Once

```bash
go build -o /usr/local/bin/bluecollar ./cmd/bluecollar
ollama serve &                 # or any OpenAI-compatible endpoint
ollama pull qwen3:4b
```

Terminal-Bench needs a running Docker daemon. `litellm` 1.95 fails to build on
a rustc older than its Rust bridge wants, so pin it:

```bash
uvx --from terminal-bench --with 'litellm==1.77.0' tb --help
```

## A run

```bash
uvx --from terminal-bench --with 'litellm==1.77.0' tb run \
  --dataset terminal-bench-core==0.1.1 \
  --agent-import-path bench.terminalbench.bluecollar_agent:BluecollarAgent \
  --model ollama/qwen3:4b \
  --task-id hello-world
```

`BLUECOLLAR_BINARY`, `BLUECOLLAR_MODEL_ENDPOINT` and `BLUECOLLAR_TIMEOUT_SECOND`
override the binary, the endpoint and the per-task budget.

Each task leaves `bluecollar-ledger.txt` and `bluecollar-metrics.json` in the
run's logging directory. The metrics file is `bench.RunMetrics`, so a suite's
worth of them summarises through `bench.SuiteReport` and lands on the same row
as any other harness measured the same way.

## Comparing against another harness

Hold the model, the dataset and the task list fixed, change only
`--agent-import-path`, and the difference is the harness. Reading a pass rate
alone hides the part that separates two harnesses on one model, which is what
each put in front of it — that figure is `promptTokensPerTurn`.

`pi_agent.py` puts [pi](https://github.com/earendil-works/pi) on the same row:

```bash
OPENAI_BASE_URL=http://host.docker.internal:11434/v1 OPENAI_API_KEY=ollama \
uvx --from terminal-bench --with 'litellm==1.77.0' tb run \
  --dataset terminal-bench-core==0.1.1 \
  --agent-import-path bench.terminalbench.pi_agent:PiAgent \
  --model qwen3:4b \
  --task-id hello-world
```

The two agents reach the model from different sides, and getting this wrong
silently measures a harness that never spoke to a model. bluecollar runs on the
host and reaches only the container's shell, so it uses the endpoint directly.
pi is installed inside the container, so it needs a base URL the container can
resolve: `host.docker.internal`, not `127.0.0.1`.
