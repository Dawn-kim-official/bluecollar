// Package model is the port a language model reaches the harness through.
//
// LanguageModelProvider is deliberately small: generate a response, or generate
// one constrained to a schema. Anything satisfying it works, and the provider
// may change between steps of a running turn — the tier ladder relies on that,
// escalating a task from a cheap model to a strong one without restarting it.
//
// There is no provider implementation here. The host brings one.
package model
