package tool

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRunShell_Proposal(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterRunShell(reg, t.TempDir()); err != nil {
		t.Fatalf("register run_shell: %v", err)
	}

	args := RunShellArgs{Shell: true, Command: "echo hi"}
	raw, _ := json.Marshal(args)
	execRes, err := reg.InvokeToolCall(context.Background(), &ToolCall{Name: "run_shell", Args: raw})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if execRes.Safe {
		t.Fatalf("proposal should not be safe=true")
	}
	if execRes.Diff == "" || execRes.Content == "" {
		t.Fatalf("expected non-empty summary and diff")
	}
}

func TestApplyShell_Echo(t *testing.T) {
	workspace := t.TempDir()
	reg := NewRegistry()
	if err := RegisterApplyShell(reg, workspace); err != nil {
		t.Fatalf("register apply_shell: %v", err)
	}

	args := ApplyShellArgs{Shell: true, Command: "echo hello"}
	raw, _ := json.Marshal(args)
	res, err := reg.Invoke(context.Background(), "apply_shell", raw)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	sr, ok := res.(*ShellResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", res)
	}
	if sr.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d, stderr=%q", sr.ExitCode, sr.Stderr)
	}
	if sr.Stdout == "" {
		t.Fatalf("expected stdout from echo")
	}
}
