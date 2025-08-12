package adapter

import "testing"

func TestParseAndString(t *testing.T) {
	m, err := ParseModel("openai:gpt-4o")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.ProviderPrefix != "openai" || m.ID != "gpt-4o" {
		t.Fatalf("unexpected model: %+v", m)
	}
	if m.String() != "openai:gpt-4o" {
		t.Fatalf("string mismatch: %s", m.String())
	}
	if _, err := ParseModel("badformat"); err == nil {
		t.Fatalf("expected error for bad format")
	}
}

func TestGetProviderFromModel(t *testing.T) {
	prov, id, err := GetProviderFromModel("claude:sonnet-3.5")
	if err != nil || prov != ProviderAnthropic || id != "sonnet-3.5" {
		t.Fatalf("anthropic mapping failed: prov=%s id=%s err=%v", prov, id, err)
	}
	prov, id, err = GetProviderFromModel("openai:gpt-4o")
	if err != nil || prov != ProviderOpenAI || id != "gpt-4o" {
		t.Fatalf("openai mapping failed: prov=%s id=%s err=%v", prov, id, err)
	}
	prov, id, err = GetProviderFromModel("ollama:llama3.1:8b")
	if err != nil || prov != ProviderOllama || id != "llama3.1:8b" {
		t.Fatalf("ollama mapping failed: prov=%s id=%s err=%v", prov, id, err)
	}
	if _, _, err := GetProviderFromModel("unknown:foo"); err == nil {
		t.Fatalf("expected error for unknown provider")
	}
}
