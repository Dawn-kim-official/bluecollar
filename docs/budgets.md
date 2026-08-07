# What a task is allowed to spend

A task needs a ceiling for two opposite reasons, and both matter equally. Work
that is going nowhere must not be allowed to grind, and work that is going
somewhere must not be cut off for looking slow. Getting one right at the cost
of the other is not a win.

## Time is the ceiling, and the model owns almost all of it

Across 147 measured runs the model's own latency was **85% to 97% of the
task's wall clock**. Tool execution, the shell, the loop's own bookkeeping —
all of it together is the remainder. Whatever a task spends its time on, it
spends it waiting for the model.

So a ceiling written as a fixed number of minutes is really a ceiling on how
much model work the task may ask for, converted at some assumed rate. Change
the model and the same minutes buy a different amount of work. The next two
sections are about what that rate actually is, because it is not the one
number it looks like.

## Measuring throughput is fitting two constants, not reading one number

A model's speed is not a single rate. Every call pays a fixed cost — the
connection, the queue, the wait for the first token — and then a per-token
cost while it generates. Divide total tokens by total wait and the two blend
into a figure that changes with how a task happens to be shaped.

Fit them instead. Every run already records the three numbers needed:

    model wait  ≈  a × model calls  +  b × output tokens

`a` is what a round trip costs and `1/b` is the real generation rate. Fitted
over runs measured here:

| model | per call | generation | prefill |
|---|---|---|---|
| google/gemini-3.1-flash-lite | 899 ms | 339 tok/s | free, cached |
| openai/gpt-5.6-luna | 3,383 ms | 596 tok/s | 6,426 tok/s |

The fitted generation rate lands within ten percent of the figure the vendor
publishes for the same model, which is the check that the fit means something.
The blended rate does not: dividing tokens by wait gives 127 and 65 tok/s for
these two, numbers that describe neither the model nor the task.

## Which means the work unit for time is the call, not the token

A successful run's median is 8 model calls at roughly 300 output tokens each.
On gpt-5.6-luna that is 27 seconds of round trips against 4 seconds of
generation: **87% of a task's time is the cost of asking, not the cost of
answering.** One call is worth about two thousand output tokens.

So an iteration or tool call budget is a good proxy for time, because calls
and turns move together. A token budget is not — it prices the cheap half.

For money the ordering reverses. Cost is prompt plus output tokens against the
price sheet, and the number of calls does not appear, except that each one
resends a prompt that keeps growing.

The device is where these constants should differ most. There is no network to
pay for, so the fixed cost should fall, while a small quantised model on an
8GB Jetson generates far slower, so the per-token cost should rise. Whether it
does is unmeasured. Fitting two constants is what makes both cases expressible
at all; a single tokens-per-second figure can describe only one of them.

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
