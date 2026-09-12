package datasyncui

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

func sortLogEntries(items []uiapi.LogEntry) {
	sort.SliceStable(items, func(i, j int) bool {
		left, _ := time.Parse(time.RFC3339Nano, items[i].Time)
		right, _ := time.Parse(time.RFC3339Nano, items[j].Time)
		return left.After(right)
	})
}

func runtimeLogEntry(line string) (uiapi.LogEntry, bool) {
	var record map[string]json.RawMessage
	if json.Unmarshal([]byte(line), &record) != nil {
		return uiapi.LogEntry{}, false
	}
	var at, level, module, message string
	_ = json.Unmarshal(record["time"], &at)
	_ = json.Unmarshal(record["level"], &level)
	_ = json.Unmarshal(record["worker"], &module)
	_ = json.Unmarshal(record["msg"], &message)
	parsed, err := time.Parse(time.RFC3339Nano, at)
	if err != nil || level == "" || message == "" {
		return uiapi.LogEntry{}, false
	}
	if module == "" {
		module = "sync-agent"
	}
	for _, key := range []string{"action", "event_id", "batch_count", "elapsed_ms", "retry_after_ms", "consecutive_errors", "phase_ms", "error", "recovered_after_errors"} {
		value, ok := record[key]
		if !ok || string(value) == `""` {
			continue
		}
		var text string
		if key == "phase_ms" {
			var phases map[string]float64
			if json.Unmarshal(value, &phases) != nil {
				continue
			}
			keys := make([]string, 0, len(phases))
			for phase := range phases {
				keys = append(keys, phase)
			}
			sort.Strings(keys)
			parts := make([]string, 0, len(keys))
			for _, phase := range keys {
				parts = append(parts, fmt.Sprintf("%s=%g", phase, phases[phase]))
			}
			text = "[" + strings.Join(parts, ",") + "]"
		} else if json.Unmarshal(value, &text) != nil {
			text = string(value)
		}
		message += " " + key + "=" + text
	}
	return uiapi.LogEntry{Time: uiapi.TimeString(parsed), Level: strings.ToUpper(level), Module: module, Message: message}, true
}

func logMatches(entry uiapi.LogEntry, req uiapi.LogQuery) bool {
	return (req.Level == "" || strings.EqualFold(entry.Level, req.Level)) && (req.Module == "" || entry.Module == req.Module)
}

func (a *App) agentLogEntries(req uiapi.LogQuery, limit int) []uiapi.LogEntry {
	if limit <= 0 {
		return nil
	}
	items := make([]uiapi.LogEntry, 0, limit)
	redact := agentlog.ForConfig(a.config)
	path := agentlog.Path(a.effectiveConfigPath())
	for backup := 0; backup <= 4 && len(items) < limit; backup++ {
		file := path
		if backup > 0 {
			file = fmt.Sprintf("%s.%d", path, backup)
		}
		lines, err := agentlog.TailFiltered(file, limit-len(items), func(line string) bool { entry, ok := runtimeLogEntry(line); return ok && logMatches(entry, req) })
		if err != nil {
			continue
		}
		for i := len(lines) - 1; i >= 0; i-- {
			entry, _ := runtimeLogEntry(lines[i])
			entry.Message = redact(entry.Message)
			items = append(items, entry)
		}
	}
	if len(items) >= limit {
		return items
	}
	legacy := agentLogPath(a.effectiveConfigPath())
	info, err := os.Stat(legacy)
	if err != nil {
		return items
	}
	parseLegacy := func(line string) uiapi.LogEntry {
		level := "INFO"
		lower := strings.ToLower(line)
		if strings.Contains(lower, "failed") || strings.Contains(lower, "error") {
			level = "ERROR"
		} else if strings.Contains(lower, "warning") {
			level = "WARN"
		}
		return uiapi.LogEntry{Time: uiapi.TimeString(info.ModTime()), Level: level, Module: "sync-agent", Message: redact(line)}
	}
	lines, err := agentlog.TailFiltered(legacy, limit-len(items), func(line string) bool { return logMatches(parseLegacy(line), req) })
	if err == nil {
		for i := len(lines) - 1; i >= 0; i-- {
			items = append(items, parseLegacy(lines[i]))
		}
	}
	return items
}
