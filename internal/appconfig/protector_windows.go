//go:build windows

package appconfig

import (
	"encoding/base64"
	"unsafe"

	"golang.org/x/sys/windows"
)

type dpapiProtector struct{}

func DefaultSecretProtector() SecretProtector {
	return dpapiProtector{}
}

func (dpapiProtector) Protect(plain string) (string, error) {
	in := bytesToBlob([]byte(plain))
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return base64.StdEncoding.EncodeToString(blobToBytes(out)), nil
}

func (dpapiProtector) Unprotect(cipher string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(cipher)
	if err != nil {
		return "", err
	}
	in := bytesToBlob(raw)
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return string(blobToBytes(out)), nil
}

func bytesToBlob(data []byte) windows.DataBlob {
	if len(data) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
}

func blobToBytes(blob windows.DataBlob) []byte {
	if blob.Size == 0 || blob.Data == nil {
		return nil
	}
	return unsafe.Slice(blob.Data, blob.Size)
}
