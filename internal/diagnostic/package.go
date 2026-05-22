package diagnostic

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type File struct {
	Name string
	Data []byte
}

func CreatePackage(dir, nodeID string, files []File) (string, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	if nodeID == "" {
		nodeID = "local"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create diagnostic directory: %w", err)
	}
	name := fmt.Sprintf("nodebridge-diagnostic-%s-%s.zip", nodeID, time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	handle, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create diagnostic package: %w", err)
	}
	defer handle.Close()

	archive := zip.NewWriter(handle)
	defer archive.Close()
	for _, file := range files {
		if file.Name == "" {
			continue
		}
		writer, err := archive.Create(file.Name)
		if err != nil {
			return "", fmt.Errorf("add diagnostic file %s: %w", file.Name, err)
		}
		if _, err := writer.Write(file.Data); err != nil {
			return "", fmt.Errorf("write diagnostic file %s: %w", file.Name, err)
		}
	}
	return path, nil
}

func JSONFile(name string, value any) (File, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return File{}, fmt.Errorf("encode diagnostic %s: %w", name, err)
	}
	return File{Name: name, Data: append(data, '\n')}, nil
}
