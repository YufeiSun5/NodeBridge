package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAtomicJSONRetriesReaderLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "progress.json")
	if err := atomicJSON(path, map[string]int{"tick": 1}); err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	released := make(chan struct{})
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = reader.Close()
		close(released)
	}()
	if err := atomicJSON(path, map[string]int{"tick": 2}); err != nil {
		t.Fatal(err)
	}
	<-released
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]int
	if err := json.Unmarshal(data, &p); err != nil || p["tick"] != 2 {
		t.Fatalf("replacement is not complete: %s, %v", data, err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains: %v", err)
	}
}

func TestAtomicJSONKeepsPreviousProgressOnPersistentLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "progress.json")
	if err := atomicJSON(path, 1); err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := atomicJSON(path, 2); err == nil {
		t.Fatal("permanent reader lock should fail")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "1" {
		t.Fatalf("previous progress not preserved: %s, %v", data, err)
	}
}
