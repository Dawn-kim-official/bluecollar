// Package bluecollar is an agent harness: the loop that takes a request,
// decides what to do, calls tools, and answers.
//
// It is built for work nobody is watching. A request arrives from someone else,
// the person who sent it goes back to their day, and the answer has to be right
// without anyone checking. That assumption is why the loop carries an outcome
// contract agreed before the work starts, a completion gate that will not accept
// the model's own word that it is done, approval as a state a task can sit in
// for days and resume from, a tier ladder that picks the model from the
// difficulty, and failure text written for the person who asked.
//
// The harness owns no tools, no identity, and no storage. A host hands it a
// tool set and a task store and calls RunTurn; every tool call executes back in
// the host, as whoever asked for the work. See [agentcontract] for the port both
// sides compile against.
//
// The harness has no identity or filesystem layout of its own either. The host
// supplies AgentIdentity, the workspace paths, the instruction bundle and the
// company context; given none of them, the agent is "the assistant" and knows
// nothing about where it runs.
package loop
