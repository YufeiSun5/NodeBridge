package diagnostic_test

import (
	"archive/zip"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/diagnostic"
)

func TestCreatePackageWritesExpectedFiles(t *testing.T) {
	jsonFile, err := diagnostic.JSONFile("config.redacted.json", map[string]string{"password": "******"})
	if err != nil {
		t.Fatalf("JSONFile returned error: %v", err)
	}
	path, err := diagnostic.CreatePackage(t.TempDir(), "edge-001", []diagnostic.File{
		jsonFile,
		{Name: "", Data: []byte("ignored")},
		{Name: "logs.json", Data: []byte("[]\n")},
	})
	if err != nil {
		t.Fatalf("CreatePackage returned error: %v", err)
	}
	if !strings.Contains(path, "edge-001") {
		t.Fatalf("expected node id in package path, got %q", path)
	}

	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer reader.Close()
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	if !names["config.redacted.json"] || !names["logs.json"] {
		t.Fatalf("missing expected diagnostic files: %+v", names)
	}
	if names[""] {
		t.Fatal("empty diagnostic file name should be skipped")
	}
}

func TestJSONFileRejectsUnsupportedValue(t *testing.T) {
	if _, err := diagnostic.JSONFile("bad.json", make(chan int)); err == nil {
		t.Fatal("expected JSONFile to reject unsupported value")
	}
}
