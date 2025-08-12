package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile_BasicNumbering(t *testing.T) {
	workspace := t.TempDir()
	abs := filepath.Join(workspace, "a.txt")
	if err := os.WriteFile(abs, []byte("x\ny"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	reg := NewRegistry()
	if err := RegisterReadFile(reg, workspace); err != nil {
		t.Fatalf("register read_file: %v", err)
	}

	args := ReadFileArgs{Path: "a.txt"}
	raw, _ := json.Marshal(args)
	res, err := reg.Invoke(context.Background(), "read_file", raw)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	r, ok := res.(*ReadFileResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", res)
	}
	if !strings.HasPrefix(r.Content, "L1: x\nL2: y") {
		t.Fatalf("unexpected content numbering: %q", r.Content)
	}
	if r.Lines != 2 {
		t.Fatalf("expected 2 lines, got %d", r.Lines)
	}
}

func TestReadFile_OffsetLimitAndRaw(t *testing.T) {
	workspace := t.TempDir()
	abs := filepath.Join(workspace, "b.txt")
	if err := os.WriteFile(abs, []byte("l1\nl2\nl3\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	reg := NewRegistry()
	if err := RegisterReadFile(reg, workspace); err != nil {
		t.Fatalf("register read_file: %v", err)
	}

	include := false
	args := ReadFileArgs{Path: "b.txt", Offset: 1, Limit: 1, IncludeLineNumbers: &include}
	raw, _ := json.Marshal(args)
	res, err := reg.Invoke(context.Background(), "read_file", raw)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	r, ok := res.(*ReadFileResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", res)
	}
	if strings.TrimSpace(r.Content) != "l2" {
		t.Fatalf("unexpected slice content: %q", r.Content)
	}
}
