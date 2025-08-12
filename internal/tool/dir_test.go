package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestListDir_BasicAndFile(t *testing.T) {
	workspace := t.TempDir()
	// Create entries
	if err := os.MkdirAll(filepath.Join(workspace, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".hidden"), []byte("h"), 0o644); err != nil {
		t.Fatalf("write hidden: %v", err)
	}

	reg := NewRegistry()
	if err := RegisterListDir(reg, workspace); err != nil {
		t.Fatalf("register list_dir: %v", err)
	}

	// List directory
	raw, _ := json.Marshal(ListDirArgs{Path: "."})
	res, err := reg.Invoke(context.Background(), "list_dir", raw)
	if err != nil {
		t.Fatalf("invoke list_dir: %v", err)
	}
	lr := res.(*ListDirResult)
	if !lr.IsDir {
		t.Fatalf("expected directory listing")
	}
	// Expect two entries: sub (dir) and file.txt (file), hidden is skipped
	if len(lr.Entries) != 2 {
		t.Fatalf("unexpected entries count: %d", len(lr.Entries))
	}
	if !(lr.Entries[0].IsDir && lr.Entries[0].Name == "sub") {
		t.Fatalf("expected first entry to be sub dir: %+v", lr.Entries[0])
	}
	if lr.Entries[1].IsDir || lr.Entries[1].Name != "file.txt" {
		t.Fatalf("expected second entry to be file.txt: %+v", lr.Entries[1])
	}

	// Query a file path
	raw2, _ := json.Marshal(ListDirArgs{Path: "file.txt"})
	res2, err := reg.Invoke(context.Background(), "list_dir", raw2)
	if err != nil {
		t.Fatalf("invoke file: %v", err)
	}
	lr2 := res2.(*ListDirResult)
	if lr2.IsDir {
		t.Fatalf("expected file info, not dir")
	}
	if len(lr2.Entries) != 1 || lr2.Entries[0].Name != "file.txt" {
		t.Fatalf("unexpected file entry: %+v", lr2.Entries)
	}
}
