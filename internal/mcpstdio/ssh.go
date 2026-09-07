package mcpstdio

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// EncodedCommand preserves Windows paths and prevents interpretation by either SSH shell.
func SSHArguments(host string, port int, key, executable string, args []string) ([]string, error) {
	if strings.HasPrefix(host, "-") || strings.ContainsAny(host, " \t\r\n") || host == "" {
		return nil, fmt.Errorf("invalid SSH target")
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid SSH port")
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	command := "[Console]::InputEncoding = [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding; & " + quote(executable)
	for _, arg := range args {
		command += " " + quote(arg)
	}
	command += "; exit $LASTEXITCODE"
	units := utf16.Encode([]rune(command))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[i*2:], unit)
	}
	result := []string{"-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-p", strconv.Itoa(port)}
	if key != "" {
		result = append(result, "-i", key)
	}
	return append(result, host, "powershell.exe -NoLogo -NoProfile -NonInteractive -EncodedCommand "+base64.StdEncoding.EncodeToString(data)), nil
}
