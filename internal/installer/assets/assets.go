package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ComponentErlang   = "erlang"
	ComponentJava     = "java"
	ComponentRabbitMQ = "rabbitmq"
	ComponentCanal    = "canal"
	ComponentWinSW    = "winsw"

	StatusOK           = "ok"
	StatusMissing      = "missing"
	StatusHashMismatch = "hash_mismatch"
	StatusInvalid      = "invalid"
	StatusSkipped      = "skipped"
)

type Catalog struct {
	Version string      `json:"version,omitempty"`
	Assets  []AssetSpec `json:"assets"`
}

type AssetSpec struct {
	Name        string   `json:"name"`
	Component   string   `json:"component"`
	Path        string   `json:"path"`
	Version     string   `json:"version,omitempty"`
	SHA256      string   `json:"sha256"`
	InstallArgs []string `json:"install_args,omitempty"`
	Optional    bool     `json:"optional,omitempty"`
}

type ValidationResult struct {
	Name           string `json:"name"`
	Component      string `json:"component"`
	Path           string `json:"path"`
	Version        string `json:"version,omitempty"`
	SizeBytes      int64  `json:"size_bytes,omitempty"`
	SHA256Expected string `json:"sha256_expected,omitempty"`
	SHA256Actual   string `json:"sha256_actual,omitempty"`
	Status         string `json:"status"`
	OK             bool   `json:"ok"`
	Message        string `json:"message,omitempty"`
}

type CommandStep struct {
	Component     string   `json:"component"`
	Action        string   `json:"action"`
	Path          string   `json:"path,omitempty"`
	Args          []string `json:"args,omitempty"`
	CommandLine   string   `json:"command_line"`
	RequiresAdmin bool     `json:"requires_admin"`
	Status        string   `json:"status"`
}

func LoadCatalog(path string) (Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, err
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return Catalog{}, err
	}
	if len(catalog.Assets) == 0 {
		return Catalog{}, fmt.Errorf("assets catalog has no assets")
	}
	return catalog, nil
}

func ValidateCatalog(catalog Catalog) []ValidationResult {
	results := make([]ValidationResult, 0, len(catalog.Assets))
	for _, asset := range catalog.Assets {
		results = append(results, ValidateAsset(asset))
	}
	return results
}

func ValidateAsset(asset AssetSpec) ValidationResult {
	result := ValidationResult{
		Name:           asset.Name,
		Component:      strings.ToLower(strings.TrimSpace(asset.Component)),
		Path:           asset.Path,
		Version:        asset.Version,
		SHA256Expected: normalizeHash(asset.SHA256),
	}
	if result.Name == "" || result.Component == "" || strings.TrimSpace(asset.Path) == "" || result.SHA256Expected == "" {
		result.Status = StatusInvalid
		result.Message = "name, component, path, and sha256 are required"
		return result
	}
	actual, size, err := fileSHA256(asset.Path)
	if err != nil {
		if os.IsNotExist(err) {
			if asset.Optional {
				result.Status = StatusSkipped
				result.OK = true
				result.Message = "optional asset file not found"
				return result
			}
			result.Status = StatusMissing
			result.Message = "asset file not found"
			return result
		}
		result.Status = StatusInvalid
		result.Message = err.Error()
		return result
	}
	result.SizeBytes = size
	result.SHA256Actual = actual
	if result.SHA256Actual != result.SHA256Expected {
		result.Status = StatusHashMismatch
		result.Message = "sha256 mismatch"
		return result
	}
	result.Status = StatusOK
	result.OK = true
	return result
}

func AllValid(results []ValidationResult) bool {
	if len(results) == 0 {
		return false
	}
	for _, result := range results {
		if !result.OK {
			return false
		}
	}
	return true
}

func BuildCommandPlan(catalog Catalog) []CommandStep {
	steps := make([]CommandStep, 0, len(catalog.Assets)+2)
	for _, asset := range catalog.Assets {
		component := strings.ToLower(strings.TrimSpace(asset.Component))
		switch component {
		case ComponentErlang:
			args := defaultArgs(asset.InstallArgs, []string{"/S"})
			steps = append(steps, CommandStep{
				Component:     component,
				Action:        "install",
				Path:          asset.Path,
				Args:          args,
				CommandLine:   commandLine(asset.Path, args),
				RequiresAdmin: true,
				Status:        "planned",
			})
		case ComponentJava:
			args := defaultArgs(asset.InstallArgs, []string{"/quiet"})
			if isArchive(asset.Path) {
				steps = append(steps, CommandStep{
					Component:     component,
					Action:        "extract",
					Path:          asset.Path,
					Args:          defaultArgs(asset.InstallArgs, []string{"-Destination", `%ProgramData%\NodeBridge\java`}),
					CommandLine:   archiveExtractCommandLine(asset.Path, defaultArgs(asset.InstallArgs, []string{"-Destination", `%ProgramData%\NodeBridge\java`})),
					RequiresAdmin: true,
					Status:        "planned",
				})
				continue
			}
			path, commandArgs := installerExecutable(asset.Path, args)
			steps = append(steps, CommandStep{
				Component:     component,
				Action:        "install",
				Path:          path,
				Args:          commandArgs,
				CommandLine:   commandLine(path, commandArgs),
				RequiresAdmin: true,
				Status:        "planned",
			})
		case ComponentRabbitMQ:
			args := defaultArgs(asset.InstallArgs, []string{"/S"})
			steps = append(steps, CommandStep{
				Component:     component,
				Action:        "install",
				Path:          asset.Path,
				Args:          args,
				CommandLine:   commandLine(asset.Path, args),
				RequiresAdmin: true,
				Status:        "planned",
			})
			steps = append(steps, rabbitMQServiceSteps()...)
		case ComponentCanal:
			args := defaultArgs(asset.InstallArgs, []string{"-Destination", `%ProgramData%\NodeBridge\canal`})
			steps = append(steps, CommandStep{
				Component:     component,
				Action:        "extract",
				Path:          asset.Path,
				Args:          args,
				CommandLine:   archiveExtractCommandLine(asset.Path, args),
				RequiresAdmin: true,
				Status:        "planned",
			})
		case ComponentWinSW:
			steps = append(steps, CommandStep{
				Component:     component,
				Action:        "copy-service-wrapper",
				Path:          asset.Path,
				CommandLine:   commandLine("Copy-Item", []string{quote(asset.Path), quote(`%ProgramData%\NodeBridge\canal\NodeBridgeCanal.exe`)}),
				RequiresAdmin: true,
				Status:        "planned",
			})
		default:
			steps = append(steps, CommandStep{
				Component:   component,
				Action:      "unsupported",
				Path:        asset.Path,
				CommandLine: "",
				Status:      "invalid",
			})
		}
	}
	return steps
}

func installerExecutable(path string, args []string) (string, []string) {
	if strings.EqualFold(filepath.Ext(path), ".msi") {
		commandArgs := append([]string{"/i", path}, args...)
		return "msiexec.exe", commandArgs
	}
	return path, args
}

func isArchive(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz")
}

func fileSHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", 0, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), info.Size(), nil
}

func normalizeHash(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func defaultArgs(args, fallback []string) []string {
	if len(args) > 0 {
		return append([]string(nil), args...)
	}
	return append([]string(nil), fallback...)
}

func rabbitMQServiceSteps() []CommandStep {
	rabbitmqctl := `%ProgramFiles%\RabbitMQ Server\rabbitmq_server\sbin\rabbitmq-service.bat`
	if runtime.GOOS != "windows" {
		rabbitmqctl = "rabbitmq-service"
	}
	return []CommandStep{
		{
			Component:     ComponentRabbitMQ,
			Action:        "service-install",
			Path:          rabbitmqctl,
			Args:          []string{"install"},
			CommandLine:   commandLine(rabbitmqctl, []string{"install"}),
			RequiresAdmin: true,
			Status:        "planned",
		},
		{
			Component:     ComponentRabbitMQ,
			Action:        "service-start",
			Path:          rabbitmqctl,
			Args:          []string{"start"},
			CommandLine:   commandLine(rabbitmqctl, []string{"start"}),
			RequiresAdmin: true,
			Status:        "planned",
		},
	}
}

func commandLine(executable string, args []string) string {
	parts := []string{quote(filepath.Clean(executable))}
	for _, arg := range args {
		parts = append(parts, quote(arg))
	}
	return strings.Join(parts, " ")
}

func archiveExtractCommandLine(path string, args []string) string {
	lower := strings.ToLower(path)
	destination := `%ProgramData%\NodeBridge\canal`
	for i := 0; i < len(args)-1; i++ {
		if strings.EqualFold(args[i], "-Destination") {
			destination = args[i+1]
			break
		}
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return commandLine("tar.exe", []string{"-xzf", path, "-C", destination})
	}
	return commandLine("Expand-Archive", append([]string{quote(path)}, args...))
}

func quote(value string) string {
	if value == "" {
		return `""`
	}
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return value
	}
	if strings.ContainsAny(value, " \t") || strings.Contains(value, `%`) || strings.Contains(value, `\`) {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}
