package agentlog

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	urlCredentials = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^\s/@]+@`)
	dsnCredentials = regexp.MustCompile(`[^\s:]+:[^\s@]+@(tcp|unix)\(`)
	quotedValue    = regexp.MustCompile(`'(?:''|\\.|[^'])*'|"(?:\\.|[^"])*"`)
	keySecret      = regexp.MustCompile(`(?i)(password|passwd|token|secret|authorization)\s*[:=]\s*[^\s,;]+`)
)

// Redactor removes configured secrets and quoted SQL values before truncation.
func Redactor(secrets ...string) func(string) string {
	unique := map[string]bool{}
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		unique[secret], unique[url.QueryEscape(secret)], unique[url.PathEscape(secret)] = true, true, true
		encoded, _ := json.Marshal(secret)
		unique[string(encoded[1:len(encoded)-1])] = true
	}
	values := make([]string, 0, len(unique))
	for value := range unique {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	pairs := make([]string, 0, len(values)*2)
	for _, value := range values {
		pairs = append(pairs, value, "[redacted]")
	}
	replacer := strings.NewReplacer(pairs...)
	return func(message string) string {
		message = replacer.Replace(message)
		message = urlCredentials.ReplaceAllString(message, `${1}[redacted]@`)
		message = dsnCredentials.ReplaceAllString(message, `[redacted]@${1}(`)
		message = quotedValue.ReplaceAllString(message, "[redacted]")
		message = keySecret.ReplaceAllString(message, `${1}=[redacted]`)
		if len(message) > 8192 {
			message = message[:8192] + " [truncated]"
		}
		return strings.ToValidUTF8(message, "?")
	}
}

func New(path string, fallback io.Writer, redact func(string) string) (*slog.Logger, io.Closer, error) {
	writer, err := openRotating(path, 8<<20, 4, fallback)
	if err != nil {
		return nil, nil, err
	}
	if redact == nil {
		redact = Redactor()
	}
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		if err, ok := attr.Value.Any().(error); ok {
			attr.Value = slog.StringValue(redact(err.Error()))
		} else if attr.Value.Kind() == slog.KindString {
			attr.Value = slog.StringValue(redact(attr.Value.String()))
		}
		return attr
	}})
	return slog.New(handler), writer, nil
}

type redactingWriter struct {
	mu     sync.Mutex
	writer io.Writer
	redact func(string) string
}

func RedactingWriter(writer io.Writer, redact func(string) string) io.Writer {
	return &redactingWriter{writer: writer, redact: redact}
}

func (w *redactingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := fmt.Fprint(w.writer, w.redact(string(p)))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
