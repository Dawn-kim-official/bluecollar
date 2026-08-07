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

## What the rows have said so far

Three benchmarks, `google/gemini-3.1-flash-lite` on both harnesses, one
attempt each, small task samples. Run-to-run variance on this model is one to
two tasks out of eight, so nothing here separates the harnesses by less than
that.

| benchmark | tasks | bluecollar | pi |
|---|---|---|---|
| terminal-bench-core | 8 | 2 | 5 |
| quixbugs | 6 | 0 | 4 |
| aider-polyglot | 6 | 1 | 3 |

pi is ahead. One row reads differently underneath: on quixbugs every
functional test passed for all six of bluecollar's runs — it found and fixed
each bug — and all six failed only `test_one_line_change`, because it rewrote
the file instead of copying it and changing one line. pi solved four
outright. On finding the bug bluecollar was 6/6 against pi's 4/6; on minimal
diff discipline it was 0/6.

## Reading a run back

```bash
bench/terminalbench/explain-run /tmp/bench-quixbugs5            # every task
bench/terminalbench/explain-run /tmp/bench-quixbugs5 bitcount   # one of them
```

For each task it says whether it was solved, how many of the benchmark's own
assertions passed and which ones did not, how many turns it took, which tools
it reached for, why it stopped, what the runtime refused it, and where the
full ledger sits. A pass rate cannot tell you that nine of ten assertions
passed and the tenth was about diff size; this can, and that is usually the
whole finding.

pi's rows read "no ledger — this harness does not record what it did", which
is the asymmetry, stated rather than hidden.

Two audiences need the same facts. The operator gets them here. The agent
holds its own observations while a turn runs, but nothing yet lets it read
its ledger back across a run — its refusals, its gate decisions, the shape of
its own thrash. That is the open half of this, and it is the half that would
let the agent diagnose itself instead of waiting for someone to read the file.

## What the measurement has found

Each of these was found by running a row, and each is invisible to a pass
rate.

- The action schema was a root-level `oneOf`, which gemini answers with `{}`
  and no error. Every run died on an empty action. The native tool path, one
  function per tool, has no root oneOf; the reference provider now takes it.
- Tool calls were decoded into a type tagged `toolCalls` while endpoints send
  `tool_calls`, so every call was dropped and the loop reported none.
- Truncation cut on byte offsets, so a cut inside a character lost a whole
  solved run's measurement.
- The completion contract required a delivery tool the task did not hold, and
  the gate asked fifteen times for the one action the task could not take.
- `completionEvidenceIDs` was a free string, so the model cited observations
  that did not exist, twenty-five turns running.
- The harness had one tool. A model asked to fix a file finished with "the
  fixed code has been saved" — a file it never wrote.
- A tool descriptor without a result contract registers without error and
  never reaches the model, which is indistinguishable from a model choosing
  not to call it.
- A result carrying effects its descriptor never declared is rejected, and
  the rejection reached the model as an ordinary retryable failure: 106 turns
  on a call that could not succeed.
- `Retryable` carried the zero value on every tool failure, so the loop told
  the model "do not retry" about failures worth retrying and enforced nothing
  about the ones that were not.

## What has not worked

Two changes were made, measured, and judged by the measurement rather than by
the reasoning behind them.

Restoring the contract's file requirement wherever a write tool existed took
the median run from 9 turns to 118 and from seven of eight runs finishing to
one. It was reverted.

Telling the agent to check its own work before finishing solved no additional
task, though it did cut the median run from 32 turns to 5 and took clean
finishes from four of six to five. What it did not do is make the agent
verify anything: `terminal_run` was still called about once per task, the
tests sitting in the container were never run, and the agent read files 210
times across six tasks instead. Whatever makes an agent check its work, a
sentence in the system instruction is not it.

## Open

The runs that fail now fail in two shapes, and both come from the same place:
bluecollar has a `finish` action a model can take at any moment, and a gate
that judges it. A weak model declares completion early — `fix-permissions`
ended after two reads — and a strict gate deadlocks. pi has neither: its run
ends when the model stops calling tools, so completion cannot be claimed, only
reached. Changing that touches the completion gate, the evidence ledger and
the reply path, which are the parts of this loop that are deliberately its
own, so it wants a design decision rather than another patch.

pi still reports only its wall clock, so the middle columns of every row above
are bluecollar's alone.
