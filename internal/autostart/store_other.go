//go:build !windows

package autostart

import "fmt"

type unsupportedStore struct{}

func DefaultStore() Store {
	return unsupportedStore{}
}

func (unsupportedStore) Get(name string) (string, bool, error) {
	return "", false, nil
}

func (unsupportedStore) Set(name, command string) error {
	return fmt.Errorf("current-user autostart is only implemented on Windows")
}

func (unsupportedStore) Delete(name string) error {
	return nil
}
