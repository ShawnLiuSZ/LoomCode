package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckpointManager_SnapshotExistingFile(t *testing.T) {
	dir := t.TempDir()
	mgr := NewCheckpointManager(dir)

	target := filepath.Join(dir, "target.txt")
	os.WriteFile(target, []byte("original content"), 0644)

	cp, err := mgr.Snapshot(target, "edit_file")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if cp.FileExisted != true {
		t.Fatal("expected FileExisted=true")
	}
	if cp.FileSize != int64(len("original content")) {
		t.Fatalf("FileSize = %d, want %d", cp.FileSize, len("original content"))
	}

	// 修改文件
	os.WriteFile(target, []byte("modified"), 0644)

	// 恢复
	if err := mgr.Restore(cp.ID); err != nil {
		t.Fatalf("restore: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(data) != "original content" {
		t.Fatalf("content = %q, want %q", string(data), "original content")
	}
}

func TestCheckpointManager_SnapshotNewFile(t *testing.T) {
	dir := t.TempDir()
	mgr := NewCheckpointManager(dir)

	target := filepath.Join(dir, "newfile.txt")

	// 文件不存在的快照
	cp, err := mgr.Snapshot(target, "write_file")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if cp.FileExisted != false {
		t.Fatal("expected FileExisted=false")
	}

	// 创建文件
	os.WriteFile(target, []byte("created"), 0644)

	// 恢复（应该删除文件）
	if err := mgr.Restore(cp.ID); err != nil {
		t.Fatalf("restore: %v", err)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted after restore, got err: %v", err)
	}
}

func TestCheckpointManager_ListSortedByTime(t *testing.T) {
	dir := t.TempDir()
	mgr := NewCheckpointManager(dir)

	target := filepath.Join(dir, "file.txt")
	os.WriteFile(target, []byte("v1"), 0644)

	cp1, _ := mgr.Snapshot(target, "edit_file")
	os.WriteFile(target, []byte("v2"), 0644)
	mgr.Snapshot(target, "edit_file")
	os.WriteFile(target, []byte("v3"), 0644)
	cp3, _ := mgr.Snapshot(target, "edit_file")

	list, err := mgr.List("", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("want 3 checkpoints, got %d", len(list))
	}
	// 最新在前
	if list[0].ID != cp3.ID {
		t.Fatalf("first = %s, want %s (newest)", list[0].ID, cp3.ID)
	}
	if list[2].ID != cp1.ID {
		t.Fatalf("last = %s, want %s (oldest)", list[2].ID, cp1.ID)
	}
}

func TestCheckpointManager_ListFilterByPath(t *testing.T) {
	dir := t.TempDir()
	mgr := NewCheckpointManager(dir)

	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	os.WriteFile(fileA, []byte("a"), 0644)
	os.WriteFile(fileB, []byte("b"), 0644)

	mgr.Snapshot(fileA, "edit_file")
	mgr.Snapshot(fileB, "edit_file")
	mgr.Snapshot(fileA, "edit_file")

	// 只查 fileA
	list, err := mgr.List(fileA, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 checkpoints for fileA, got %d", len(list))
	}
	for _, cp := range list {
		if cp.OriginalPath != fileA {
			t.Fatalf("expected path %s, got %s", fileA, cp.OriginalPath)
		}
	}
}

func TestCheckpointManager_RestoreLast(t *testing.T) {
	dir := t.TempDir()
	mgr := NewCheckpointManager(dir)

	target := filepath.Join(dir, "file.txt")
	os.WriteFile(target, []byte("v1"), 0644)
	mgr.Snapshot(target, "edit_file")
	os.WriteFile(target, []byte("v2"), 0644)
	mgr.Snapshot(target, "edit_file")
	os.WriteFile(target, []byte("v3"), 0644)

	cp, err := mgr.RestoreLast()
	if err != nil {
		t.Fatalf("restore last: %v", err)
	}
	if cp == nil {
		t.Fatal("expected checkpoint, got nil")
	}

	data, _ := os.ReadFile(target)
	if string(data) != "v2" {
		t.Fatalf("content = %q, want %q", string(data), "v2")
	}
}

func TestCheckpointManager_Eviction(t *testing.T) {
	dir := t.TempDir()
	mgr := NewCheckpointManager(dir)
	mgr.maxEntries = 3

	target := filepath.Join(dir, "file.txt")
	os.WriteFile(target, []byte("content"), 0644)

	for i := 0; i < 5; i++ {
		mgr.Snapshot(target, "edit_file")
	}

	list, _ := mgr.List("", 100)
	if len(list) > 3 {
		t.Fatalf("expected max 3 checkpoints after eviction, got %d", len(list))
	}
}

func TestCheckpointManager_RestoreNonExistent(t *testing.T) {
	dir := t.TempDir()
	mgr := NewCheckpointManager(dir)

	err := mgr.Restore("nonexistent_id")
	if err == nil {
		t.Fatal("expected error for non-existent checkpoint")
	}
}

func TestFormatCheckpointSummary_Empty(t *testing.T) {
	s := FormatCheckpointSummary(nil)
	if s != "没有可用的快照。" {
		t.Fatalf("unexpected summary: %q", s)
	}
}

func TestFormatCheckpointSummary_NonEmpty(t *testing.T) {
	dir := t.TempDir()
	mgr := NewCheckpointManager(dir)

	target := filepath.Join(dir, "file.txt")
	os.WriteFile(target, []byte("content"), 0644)
	mgr.Snapshot(target, "edit_file")

	list, _ := mgr.List("", 10)
	s := FormatCheckpointSummary(list)
	if s == "没有可用的快照。" {
		t.Fatal("expected non-empty summary")
	}
}
