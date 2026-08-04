// Package intake decides what an inbound message means before anything runs.
//
// A turn router chooses whether a message becomes a task, a quick reply, or
// nothing at all. A classifier answers the two questions a chat platform forces:
// whether a message in a busy channel is addressed to the agent, and whether it
// continues a task already running.
//
// These are host policy rather than harness behaviour — a host that answers its
// own messenger may bring its own — which is why they live beside the loop
// instead of on the harness port.
package intake
