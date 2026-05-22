package autostart

import (
	"fmt"
	"os"
)

const AppName = "NodeBridge DataSync"

type Manager interface {
	Enabled() (bool, error)
	SetEnabled(enabled bool) error
}

type Store interface {
	Get(name string) (string, bool, error)
	Set(name, command string) error
	Delete(name string) error
}

type CurrentUserManager struct {
	Store Store
	Name  string
	Path  string
}

func NewCurrentUserManager() CurrentUserManager {
	return CurrentUserManager{Store: DefaultStore(), Name: AppName}
}

func (m CurrentUserManager) Enabled() (bool, error) {
	store := m.Store
	if store == nil {
		store = DefaultStore()
	}
	name := m.name()
	_, ok, err := store.Get(name)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (m CurrentUserManager) SetEnabled(enabled bool) error {
	store := m.Store
	if store == nil {
		store = DefaultStore()
	}
	name := m.name()
	if !enabled {
		return store.Delete(name)
	}
	path := m.Path
	if path == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}
		path = exe
	}
	return store.Set(name, quoteCommand(path))
}

func (m CurrentUserManager) name() string {
	if m.Name != "" {
		return m.Name
	}
	return AppName
}

func quoteCommand(path string) string {
	return `"` + path + `"`
}
