//go:build !windows

package appconfig

import "encoding/base64"

type portableProtector struct{}

func DefaultSecretProtector() SecretProtector {
	return portableProtector{}
}

func (portableProtector) Protect(plain string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(plain)), nil
}

func (portableProtector) Unprotect(cipher string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(cipher)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
