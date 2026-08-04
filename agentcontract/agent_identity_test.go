package agentcontract

import "testing"

func TestAnAgentWithNoGivenNameIsNotNamedAfterItsHarness(t *testing.T) {
	identity := AgentIdentity{}

	if identity.DisplayName() != "the assistant" {
		t.Fatalf("a harness must not name itself to the model, got %q", identity.DisplayName())
	}
	if identity.MentionExample() != "your bot handle" {
		t.Fatalf("expected a neutral mention example, got %q", identity.MentionExample())
	}
}

func TestTheHostNamesTheAgent(t *testing.T) {
	identity := AgentIdentity{Name: "김인턴", Handle: "internkim"}

	if identity.DisplayName() != "김인턴" {
		t.Fatalf("expected the host's name, got %q", identity.DisplayName())
	}
	if identity.MentionExample() != "@internkim" {
		t.Fatalf("expected the host's handle, got %q", identity.MentionExample())
	}
}

func TestAHandleThatAlreadyCarriesItsAtSignIsNotDoubled(t *testing.T) {
	if mention := (AgentIdentity{Handle: "@bot"}).MentionExample(); mention != "@bot" {
		t.Fatalf("expected @bot, got %q", mention)
	}
}
