//go:build windows

package autostart

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

type registryStore struct{}

func DefaultStore() Store {
	return registryStore{}
}

func (registryStore) Get(name string) (string, bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return "", false, nil
		}
		return "", false, fmt.Errorf("open autostart registry: %w", err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	if err != nil {
		if err == registry.ErrNotExist {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read autostart registry: %w", err)
	}
	return value, true, nil
}

func (registryStore) Set(name, command string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open autostart registry for write: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue(name, command); err != nil {
		return fmt.Errorf("write autostart registry: %w", err)
	}
	return nil
}

func (registryStore) Delete(name string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("open autostart registry for delete: %w", err)
	}
	defer key.Close()
	if err := key.DeleteValue(name); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete autostart registry: %w", err)
	}
	return nil
}
