package engflow

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type runConfig struct {
	Feature          string
	SpecPath         string
	CatalogPath      string
	RequirementsPath string
	DesignPath       string
	ArchitecturePath string
	ArchitectureAI   string
	RepoRoot         string
	RegenCmd         string
	TestCmd          string
	WatchPaths       []string
}

func defaultRunConfig() runConfig {
	cfg := runConfig{
		Feature:          "default",
		SpecPath:         "spec.md",
		CatalogPath:      "catalog.yml",
		RequirementsPath: "requirements.yml",
		DesignPath:       "design.yml",
		ArchitecturePath: "architecture.yml",
		ArchitectureAI:   "ARCHITECTURE.ai.json",
		RepoRoot:         ".",
		RegenCmd:         "",
		TestCmd:          "",
	}
	cfg.WatchPaths = []string{cfg.CatalogPath, cfg.RequirementsPath, cfg.DesignPath, cfg.ArchitecturePath, cfg.ArchitectureAI}
	return cfg
}

func resolveConfigPath(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--config="))
		}
		if arg == "--config" && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
	}
	return ".engflow/config.yml"
}

func loadRunConfig(path string) (runConfig, error) {
	cfg := defaultRunConfig()
	path = strings.TrimSpace(path)
	if path == "" {
		path = ".engflow/config.yml"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	values := parseSimpleYAMLKV(string(b))
	for key, value := range values {
		switch key {
		case "feature", "default_feature":
			cfg.Feature = value
		case "spec", "spec_path", "paths.spec":
			cfg.SpecPath = value
		case "catalog", "catalog_path", "paths.catalog":
			cfg.CatalogPath = value
		case "requirements", "requirements_path", "paths.requirements":
			cfg.RequirementsPath = value
		case "design", "design_path", "paths.design":
			cfg.DesignPath = value
		case "architecture", "architecture_path", "paths.architecture":
			cfg.ArchitecturePath = value
		case "architecture_ai", "architecture_ai_path", "paths.architecture_ai":
			cfg.ArchitectureAI = value
		case "repo_root", "paths.repo_root":
			cfg.RepoRoot = value
		case "regen_cmd", "commands.regen", "engmodel_generate_cmd":
			cfg.RegenCmd = value
		case "test_cmd", "commands.test":
			cfg.TestCmd = value
		case "watch", "watch_paths", "verify.watch":
			cfg.WatchPaths = parseWatchList(value)
		}
	}

	cfg.Feature = firstNonEmpty(cfg.Feature, "default")
	cfg.SpecPath = firstNonEmpty(cfg.SpecPath, "spec.md")
	cfg.CatalogPath = firstNonEmpty(cfg.CatalogPath, "catalog.yml")
	cfg.RequirementsPath = firstNonEmpty(cfg.RequirementsPath, "requirements.yml")
	cfg.DesignPath = firstNonEmpty(cfg.DesignPath, "design.yml")
	cfg.ArchitecturePath = firstNonEmpty(cfg.ArchitecturePath, "architecture.yml")
	cfg.ArchitectureAI = firstNonEmpty(cfg.ArchitectureAI, "ARCHITECTURE.ai.json")
	cfg.RepoRoot = firstNonEmpty(cfg.RepoRoot, ".")
	if len(cfg.WatchPaths) == 0 {
		cfg.WatchPaths = []string{cfg.CatalogPath, cfg.RequirementsPath, cfg.DesignPath, cfg.ArchitecturePath, cfg.ArchitectureAI}
	}

	return cfg, nil
}

func parseSimpleYAMLKV(raw string) map[string]string {
	out := map[string]string{}
	currentSection := ""
	lines := strings.Split(raw, "\n")
	for _, ln := range lines {
		line := stripCommentPreservingQuotes(ln)
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := countLeadingSpaces(line)
		trimmed := strings.TrimSpace(line)

		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, ": ") {
			key := normalizeConfigKey(strings.TrimSuffix(trimmed, ":"))
			if indent == 0 {
				currentSection = key
			}
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := normalizeConfigKey(parts[0])
		value := cleanConfigValue(parts[1])
		if indent > 0 && currentSection != "" {
			key = currentSection + "." + key
		}
		out[key] = value
	}
	return out
}

func normalizeConfigKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func cleanConfigValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if (strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"")) || (strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'")) {
		if unquoted, err := strconv.Unquote(v); err == nil {
			return strings.TrimSpace(unquoted)
		}
		return strings.TrimSpace(v[1 : len(v)-1])
	}
	return v
}

func countLeadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r != ' ' {
			break
		}
		n++
	}
	return n
}

func stripCommentPreservingQuotes(s string) string {
	inSingle := false
	inDouble := false
	escape := false
	for i, r := range s {
		if escape {
			escape = false
			continue
		}
		switch r {
		case '\\':
			if inDouble {
				escape = true
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return s[:i]
			}
		}
	}
	return s
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
