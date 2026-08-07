# What a task is allowed to spend

A task needs a ceiling for two opposite reasons, and both matter equally. Work
that is going nowhere must not be allowed to grind, and work that is going
somewhere must not be cut off for looking slow. Getting one right at the cost
of the other is not a win.

## Time is the ceiling, and the model owns almost all of it

Across 147 measured runs the model's own latency was **85% to 97% of the
task's wall clock**. Tool execution, the shell, the loop's own bookkeeping —
all of it together is the remainder. A task's duration is very nearly the
tokens it generated divided by how fast the model generates them.

That has a consequence worth stating plainly: a wall-clock ceiling written as
a fixed number of minutes is really a token ceiling divided by an assumed
throughput. Change the model and the same minutes buy a different amount of
work.

## Measured throughput

Output tokens per second, measured end to end from our own runs, so network
and our own overhead are included and these sit below a vendor's figure:

| model | measured | runs |
|---|---|---|
| google/gemini-3.1-flash-lite | 127 tok/s | 80 |
| openai/gpt-5.6-luna | 65 tok/s | 67 |

Published figures for the same class of model, measured at the provider:

| model | published |
|---|---|
| Gemini 3.5 Flash-Lite | 372 tok/s |
| Gemini 3.1 Flash-Lite | 312 tok/s |
| Gemini 3.6 Flash | 200 tok/s |
| median, hosted reasoning models | 72–106 tok/s |

The number that matters most is not in either table. The device runs
gemma-4-E2B on llama.cpp on an 8GB Jetson, and a small quantised model on that
hardware is an order of magnitude below any row above. **Its throughput has
not been measured, and every duration here was set from hosted models.** Until
it is measured, the local path is running on a budget derived from machines
roughly ten times faster than it, which is the failure mode where a task is
stopped for slowness that belongs to the hardware rather than to the work.

Recommended floor when a model's throughput is unknown: **20 tok/s**. It is
low enough to cover a small local model and high enough that a ceiling built
on it is still a ceiling.

## The ladder

`bench/derive-budgets` reads the iteration and tool call budgets off runs that
succeeded: the first working tier is their 95th percentile, and each tier
doubles. Durations follow the same shape, anchored on the measured 95th
percentile of successful wall clock — 4 minutes — with a factor of two of
margin.

| tier | duration | iterations | tool calls |
|---|---|---|---|
| xlow | 3 min | 4 | 1 |
| low | 8 min | 20 | 13 |
| medium | 16 min | 40 | 26 |
| high | 32 min | 80 | 52 |
| xhigh | 64 min | 160 | 104 |
| max | 128 min | 320 | 208 |

Doubling is the point. The ladder it replaced went 40 minutes to 60 to 60, and
220 tool calls to 260 — an escalation that buys eighteen percent more is a
ceiling wearing a ladder's clothes. A tier a task escalates *into* has to be
able to finish work the tier below it could not.

## Who decides what

The model classifies the tier, because judging what kind of work a request is
takes judgement. The runtime assigns what that tier is allowed to spend,
because a model that wants to keep working should not be the one setting how
long it may work.

Intake also estimates `estimatedMinutes` — how long a careful human
professional would spend on the task. That is planning metadata and must not
become the budget: measured against real runs the estimate ran 30 to 75 times
longer than the agent actually took, because it answers a different question.
Its value is elsewhere. It says what the work is worth, and an agent that
takes longer than the person it is standing in for has stopped being useful
however correct its answer is.

## What is still owed

- The Jetson's throughput, measured rather than assumed.
- Durations derived as work over throughput instead of fixed minutes, so a
  slow model gets proportionally more time for the same work.
- Cost. Every run before today reported `costUSD` zero because the reference
  provider never asked the endpoint to account for it. It asks now, and once
  enough runs carry real cost the same percentile method applies to money.
