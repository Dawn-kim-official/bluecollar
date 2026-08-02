# bluecollar

An agent harness: the loop that takes a request, decides what to do, calls tools, and answers.

bluecollar does not own tools, identity, or storage. It is handed a tool set and a task store by a
host and runs the turn. That separation is the point — the same loop runs behind a chat connector on
a server, or in front of you in a terminal.

Status: extraction in progress. The loop currently lives inside
[blueclaw](https://github.com/Dawn-kim-official/blueclaw) at `internal/bluecollar` and is being moved
here. See "What is not here yet".

## The shape

```
host  ──── agentcontract.Harness ────  bluecollar
  │                                        │
  │ owns: tools, identity, task store,     │ owns: the turn loop, routing,
  │       approvals, POSIX isolation       │       skills, completion judgment
  │                                        │
  └──────── executes every tool call ──────┘
```

The host and the harness compile against one shared contract package,
[`blueclaw/agentcontract`](https://github.com/Dawn-kim-official/blueclaw/tree/main/agentcontract).
The host names only that interface, never this package. A different harness — an AI SDK adapter, an
external agent — drops into the same socket.

The nine verbs the host may call:

| Verb | Purpose |
|---|---|
| `RunTurn` | Run one turn of an existing task |
| `RouteTurn` | Decide what an inbound message means before running anything |
| `RunAgentRequest` | Route and run in one call |
| `CompleteLaunchFailure` | Turn a launch failure into an explanation for the person |
| `GenerateReply` | One-shot reply, no task |
| `GenerateReplyWithContext` | One-shot reply with visible context and memory |
| `ClassifyAddressing` | Decide whether a message in a channel is for us |
| `ClassifyActiveTaskFollowUp` | Decide whether a message continues a running task |
| `RefreshSkillIndex` | Re-read the skill bundle |

Tool execution never happens here. The harness decides *what* to call; the host decides *who* it runs
as. A harness that runs its own tools defeats the host's isolation boundary and is not a valid
implementation of this contract.

## Provider-agnostic

Models reach bluecollar through a provider port, not a vendor SDK. Anything satisfying it works, and
the provider can change **between steps of a running turn** — the tier ladder relies on that, escalating
a task from a cheap model to a strong one without restarting it.

The reference provider is an [AI SDK](https://ai-sdk.dev) sidecar, which is what makes "any model"
literal rather than aspirational.

## What is not here yet

Honest list, kept current:

- The loop itself. It is still in the blueclaw repo pending the module split.
- The terminal CLI. Planned on [termcn](https://github.com/shadcn-labs/termcn).
- Native multi-step tool calling. The loop currently forces one structured action per step, which
  costs a turn per tool call and blocks parallel calls. Migration is planned and staged.

## License

MIT. See [LICENSE](./LICENSE).
