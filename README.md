<img src="assets/bluecollar.logo.svg" alt="bluecollar" width="112">

# bluecollar

> **Status: pre-alpha, under active development.** The exported API, the
> contract types and the event names all still change without notice, and there
> is no release, no versioning policy and no migration path between commits. It
> is published so the design can be read and argued with, not so it can be
> depended on. If you import it, pin a commit and expect to read diffs.

An agent harness: the loop that takes a request, decides what to do, calls tools, and answers.

bluecollar does not own tools, identity, or storage. It is handed a tool set and a task store by a
host and runs the turn. That separation is the point — the same loop runs behind a chat connector on
a server, or in front of you in a terminal.

It is built for work nobody is watching. A request arrives from someone else, the person who sent it
goes back to their day, and the answer has to be right without anyone checking. That assumption is
why the loop carries things an interactive coding agent has no use for: an outcome contract agreed
before the work starts, a completion gate that will not accept the model's own word that it is done,
approval as a state a task can sit in for days and resume from, a tier ladder that picks the model
from the difficulty rather than from a flag, and failure text written for the person who asked
rather than for a log.

The trade is real and worth stating plainly: it is heavier than an interactive loop, and for sitting
beside a developer and fixing code as they watch, a coding agent is the better tool.

## The shape

```
host  ──── agentcontract.Harness ────  bluecollar
  │                                        │
  │ owns: tools, identity, task store,     │ owns: the turn loop, routing,
  │       approvals, process isolation     │       skills, completion judgment
  │                                        │
  └──────── executes every tool call ──────┘
```

The host and the harness compile against one shared contract package,
[`agentcontract`](./agentcontract). A different harness — an AI SDK adapter, an external agent —
drops into the same socket.

The port is one method:

```go
type Harness interface {
	RunTurn(context.Context, AgentTurnRequest) (AgentTurnResult, error)
}
```

It used to be nine. Routing, addressing, follow-up classification and one-shot replies were verbs on
the port until it became clear they are host policy, not harness behaviour: a host that answers
its own messenger decides what an inbound message *means* before anything runs a turn. Those
still live here — [`intake.Classifier`](./intake) routes and classifies, `AgentKernel` carries
`RunAgentRequest` and `CompleteLaunchFailure` — but a host is free to bring its own, and a harness
that implements only `RunTurn` is complete.

Tool execution never happens here. The harness decides *what* to call; the host decides *who* it runs
as. A harness that runs its own tools defeats the host's isolation boundary and is not a valid
implementation of this contract.

The harness has no identity of its own. The host supplies `AgentIdentity`, the workspace layout, the
instruction bundle and the company context; with none given, the agent is "the assistant" and knows
nothing about where it runs.

## Provider-agnostic

Models reach bluecollar through a provider port, not a vendor SDK. Anything satisfying it works, and
the provider can change **between steps of a running turn** — the tier ladder relies on that, escalating
a task from a cheap model to a strong one without restarting it.

There is no provider implementation in this module — the port is the contract, and the host brings
the provider. The reference one, in [blueclaw](https://github.com/Dawn-kim-official/blueclaw), is an
[AI SDK](https://ai-sdk.dev) sidecar, which is what makes "any model" literal rather than
aspirational.

## What is not here yet

Honest list, kept current:

- A terminal front end of its own. Planned on [termcn](https://github.com/shadcn-labs/termcn); today
  the only way to drive the loop is to embed it in a host.
- Native multi-step tool calling. The loop currently forces one structured action per step, which
  costs a turn per tool call and blocks parallel calls. Migration is planned and staged.

## Building and testing

The module depends on one library and nothing outside its own directory.

```
go build ./...
go test ./...
```

Every check that runs in CI is in [`.github/workflows/check.yml`](./.github/workflows/check.yml):
`gofmt`, `go vet`, `go build`, `go test`. No network, no credentials, no database.

## License

MIT. See [LICENSE](./LICENSE).
