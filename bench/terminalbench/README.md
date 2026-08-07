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

## What the row has said so far

Terminal-Bench `terminal-bench-core==0.1.1`, task `hello-world`, one attempt
each, the same model on both sides.

The first row used a local `gemma-4-E4B-it UD-Q4_K_XL` served by llama.cpp with
the MTP drafter. Neither harness solved it, and a pass rate alone would have
called those two failures the same result: bluecollar worked until it hit its
iteration ceiling, pi answered "I am a Large Language Model developed by Google
DeepMind" and stopped.

On `openai/gpt-5.6-luna` both harnesses solve it, and the interesting column is
no longer the verdict.

| harness | resolved | turns | tool calls | prompt tokens/turn | wall clock |
|---|---|---|---|---|---|
| bluecollar | yes | 40 | 12 | 8,643 | 234s |
| pi | yes | not reported | not reported | not reported | 21s |

bluecollar creates the file within the first few turns and then cannot stop.
Every run so far has ended at `max_iterations` rather than at a finish, which
is why `reachedEnd` is false on a task the benchmark scores as passed.

## What the measurement has found

Each of these was found by running the row, not by reading code, and each is
the kind of defect a pass rate hides completely.

- The action schema is a root-level `oneOf`, which strict structured output
  rejects, so every run 400'd before reaching a model. Fixed by asking for a
  native tool call, the way the product path already does.
- A model that answers with prose instead of the required call had its prose
  handed to a JSON parser. Now the call is forced, and prose is a named
  failure carrying the finish reason.
- Truncation cut on byte offsets, so a cut inside a multi-byte character
  produced undecodable output and lost a whole solved run's measurement.
- The outcome contract required a delivered file attachment on a task holding
  only a terminal, so the gate asked 15 times for the one action the task
  could not take.

## Open

The run still ends at the iteration ceiling. The gate now refuses finish for a
different reason: the model cites `completionEvidenceIDs` that do not match any
successful observation, 25 times in the last run. The valid IDs are known to
the runtime and finite at the moment the schema is built, so they belong in the
schema as a closed enum rather than being re-asked from the model as free
strings — which is the same rule the rest of this loop follows and the next
thing to fix.

pi reports no per-run figures, so its columns are what Terminal-Bench observed
from outside while bluecollar's come from its own ledger. That asymmetry is
worth closing before reading much into the middle columns.
