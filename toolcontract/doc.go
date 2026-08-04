// Package toolcontract describes tools to a harness and carries their results
// back.
//
// A ToolDescriptor says what a tool is called, what it accepts, and — through
// ApprovalScope and SideEffectClass — what kind of effect running it has. Those
// two fields are how a host decides which calls need a person's approval, so
// that the decision comes from what a tool does rather than from what it is
// named.
//
// ToolSet is the set of tools a particular requester may call this turn. The
// harness chooses from it; the host executes.
package toolcontract
