package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupRegistryForTests(t *testing.T, workspace string) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := RegisterEditFile(reg, workspace); err != nil {
		t.Fatalf("register edit_file: %v", err)
	}
	if err := RegisterApplyEdit(reg, workspace); err != nil {
		t.Fatalf("register apply_edit: %v", err)
	}
	return reg
}

func invokeTool(t *testing.T, reg *Registry, name string, args any) *ExecutionResult {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	call := &ToolCall{Name: name, Args: raw}
	res, err := reg.InvokeToolCall(context.Background(), call)
	if err != nil {
		t.Fatalf("invoke %s: %v", name, err)
	}
	return res
}

func mustWriteFile(t *testing.T, dir, relPath, content string) string {
	t.Helper()
	abs := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return abs
}

func readFileContent(t *testing.T, dir, relPath string) string {
	t.Helper()
	abs := filepath.Join(dir, relPath)
	b, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(b)
}

func TestEditTool_CreateAndApply(t *testing.T) {
	workspace := t.TempDir()
	reg := setupRegistryForTests(t, workspace)

	createArgs := EditFileArgs{
		Path:    "hello.txt",
		Action:  "CREATE",
		Content: "hello\nworld",
	}

	// Propose edit
	res := invokeTool(t, reg, "edit_file", createArgs)
	if res.Safe {
		t.Fatalf("expected unsafe proposal (requires approval), got safe=true")
	}
	if res.Diff == "" {
		t.Fatalf("expected a diff for proposal")
	}
	if !strings.Contains(res.Diff, "hello\nworld") {
		t.Fatalf("diff should contain new content; got: %q", res.Diff)
	}

	// Apply edit
	applyArgs := ApplyEditArgs{
		Path:    createArgs.Path,
		Action:  createArgs.Action,
		Content: createArgs.Content,
	}
	res2 := invokeTool(t, reg, "apply_edit", applyArgs)
	if !res2.Safe {
		t.Fatalf("expected apply to be safe=true")
	}
	got := readFileContent(t, workspace, createArgs.Path)
	if got != createArgs.Content {
		t.Fatalf("file content mismatch: got %q want %q", got, createArgs.Content)
	}
}

func TestEditTool_ReplaceLines(t *testing.T) {
	workspace := t.TempDir()
	reg := setupRegistryForTests(t, workspace)

	mustWriteFile(t, workspace, "rep.txt", "a\nb\nc")

	args := EditFileArgs{
		Path:      "rep.txt",
		Action:    "REPLACE",
		StartLine: 2,
		EndLine:   2,
		Content:   "BETA",
	}
	res := invokeTool(t, reg, "edit_file", args)
	if res.Diff == "" {
		t.Fatalf("expected non-empty diff for replace proposal")
	}
	// apply
	res2 := invokeTool(t, reg, "apply_edit", ApplyEditArgs{
		Path:      args.Path,
		Action:    args.Action,
		StartLine: args.StartLine,
		EndLine:   args.EndLine,
		Content:   args.Content,
	})
	if !res2.Safe {
		t.Fatalf("expected apply to be safe=true")
	}
	got := readFileContent(t, workspace, args.Path)
	want := "a\nBETA\nc"
	if got != want {
		t.Fatalf("file content mismatch: got %q want %q", got, want)
	}
}

func TestEditTool_InsertAfter(t *testing.T) {
	workspace := t.TempDir()
	reg := setupRegistryForTests(t, workspace)

	mustWriteFile(t, workspace, "ins.txt", "first\nsecond")

	args := EditFileArgs{
		Path:    "ins.txt",
		Action:  "INSERT_AFTER",
		Line:    1,
		Content: "inserted",
	}
	_ = invokeTool(t, reg, "edit_file", args)
	_ = invokeTool(t, reg, "apply_edit", ApplyEditArgs{
		Path:    args.Path,
		Action:  args.Action,
		Line:    args.Line,
		Content: args.Content,
	})
	got := readFileContent(t, workspace, args.Path)
	want := "first\ninserted\nsecond"
	if got != want {
		t.Fatalf("file content mismatch: got %q want %q", got, want)
	}
}

func TestEditTool_DeleteLines(t *testing.T) {
	workspace := t.TempDir()
	reg := setupRegistryForTests(t, workspace)

	mustWriteFile(t, workspace, "del.txt", "one\ntwo\nthree")

	args := EditFileArgs{
		Path:      "del.txt",
		Action:    "DELETE",
		StartLine: 2,
		EndLine:   3,
	}
	_ = invokeTool(t, reg, "edit_file", args)
	_ = invokeTool(t, reg, "apply_edit", ApplyEditArgs{
		Path:      args.Path,
		Action:    args.Action,
		StartLine: args.StartLine,
		EndLine:   args.EndLine,
	})
	got := readFileContent(t, workspace, args.Path)
	want := "one"
	if got != want {
		t.Fatalf("file content mismatch: got %q want %q", got, want)
	}
}

func TestEditTool_SearchReplace(t *testing.T) {
	workspace := t.TempDir()
	reg := setupRegistryForTests(t, workspace)

	mustWriteFile(t, workspace, "sr.txt", "foo\na\nfoo")

	args := EditFileArgs{
		Path:      "sr.txt",
		Action:    "SEARCH_REPLACE",
		OldString: "foo",
		NewString: "bar",
	}
	res := invokeTool(t, reg, "edit_file", args)
	if res.Diff == "" {
		t.Fatalf("expected non-empty diff for search/replace proposal")
	}
	_ = invokeTool(t, reg, "apply_edit", ApplyEditArgs{
		Path:      args.Path,
		Action:    args.Action,
		OldString: args.OldString,
		NewString: args.NewString,
	})
	got := readFileContent(t, workspace, args.Path)
	want := "bar\na\nbar"
	if got != want {
		t.Fatalf("file content mismatch: got %q want %q", got, want)
	}
}
