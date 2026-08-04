// Package taskstate is the durable record of work.
//
// A TaskRun is one unit of work with one of nine statuses, and its event ledger
// is the append-only sequence of everything that happened to it. Event names
// follow a fixed grammar — tool.<name>.requested, tool.<name>.result,
// approval.pending_call, approval.executed — so a reader can reconstruct a run
// without access to any harness's internal types.
//
// The ledger is what makes a task survive its process. A run is resumed by
// re-driving a turn from what the ledger says, never by attaching to a live one,
// which is why a task can wait days for an approval and continue afterwards.
package taskstate
