package autostart

import "testing"

func TestCurrentUserManagerSetAndClear(t *testing.T) {
	store := &memoryStore{values: map[string]string{}}
	manager := CurrentUserManager{Store: store, Name: "NodeBridge Test", Path: `C:\NodeBridge\DataSync.exe`}

	enabled, err := manager.Enabled()
	if err != nil {
		t.Fatalf("Enabled returned error: %v", err)
	}
	if enabled {
		t.Fatal("expected autostart disabled")
	}

	if err := manager.SetEnabled(true); err != nil {
		t.Fatalf("SetEnabled(true) returned error: %v", err)
	}
	if store.values["NodeBridge Test"] != `"C:\NodeBridge\DataSync.exe"` {
		t.Fatalf("unexpected command %q", store.values["NodeBridge Test"])
	}
	enabled, _ = manager.Enabled()
	if !enabled {
		t.Fatal("expected autostart enabled")
	}

	if err := manager.SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false) returned error: %v", err)
	}
	enabled, _ = manager.Enabled()
	if enabled {
		t.Fatal("expected autostart disabled after delete")
	}
}

type memoryStore struct {
	values map[string]string
}

func (s *memoryStore) Get(name string) (string, bool, error) {
	value, ok := s.values[name]
	return value, ok, nil
}

func (s *memoryStore) Set(name, command string) error {
	s.values[name] = command
	return nil
}

func (s *memoryStore) Delete(name string) error {
	delete(s.values, name)
	return nil
}
