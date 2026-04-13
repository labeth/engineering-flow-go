package engflow

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func runGate(args []string, out, errOut io.Writer) int {
	configPath := resolveConfigPath(args)
	cfg, err := loadRunConfig(configPath)
	if err != nil {
		fmt.Fprintf(errOut, "gate: load config %q: %v\n", configPath, err)
		return 1
	}

	fs := newFlagSet("gate")
	_ = fs.String("config", configPath, "path to config file")
	feature := fs.String("feature", cfg.Feature, "feature identifier")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(errOut, "gate: %v\n", err)
		return 2
	}

	featureSafe := safeFeatureName(*feature)
	fmt.Fprintln(out, "[engflow] gate: verify")
	if code := runVerify([]string{"--config", configPath, "--feature", featureSafe}, out, errOut); code != 0 {
		return code
	}

	fmt.Fprintln(out, "[engflow] gate: drift")
	if code := runDrift([]string{"--config", configPath, "--feature", featureSafe}, out, errOut); code != 0 {
		return code
	}

	fmt.Fprintln(out, "[engflow] gate: status")
	if code := runStatus([]string{"--config", configPath, "--feature", featureSafe}, out, errOut); code != 0 {
		return code
	}

	fmt.Fprintln(out, "[engflow] gate complete")
	return 0
}

type traceMatch struct {
	Path string
	Line int
	Text string
}

func runTraceQuery(args []string, out, errOut io.Writer) int {
	configPath := resolveConfigPath(args)
	cfg, err := loadRunConfig(configPath)
	if err != nil {
		fmt.Fprintf(errOut, "trace-query: load config %q: %v\n", configPath, err)
		return 1
	}

	fs := newFlagSet("trace-query")
	_ = fs.String("config", configPath, "path to config file")
	id := fs.String("id", "", "requirement identifier to search for")
	repoRoot := fs.String("repo-root", cfg.RepoRoot, "repository root to scan")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(errOut, "trace-query: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(errOut, "trace-query: --id is required")
		return 2
	}

	matches, err := collectTraceMatches(*repoRoot, strings.TrimSpace(*id))
	if err != nil {
		fmt.Fprintf(errOut, "trace-query: scan repo: %v\n", err)
		return 1
	}
	if len(matches) == 0 {
		fmt.Fprintf(out, "no matches found for %s\n", strings.TrimSpace(*id))
		return 1
	}

	for _, m := range matches {
		fmt.Fprintf(out, "%s:%d:%s\n", m.Path, m.Line, m.Text)
	}
	return 0
}

func collectTraceMatches(root, id string) ([]traceMatch, error) {
	matches := make([]traceMatch, 0, 16)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".engflow" {
				if filepath.Clean(path) == filepath.Clean(root) {
					return nil
				}
				return filepath.SkipDir
			}
			return nil
		}
		if !scanEligibleFile(path) {
			return nil
		}
		fileMatches, err := grepFile(path, id)
		if err != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path == matches[j].Path {
			return matches[i].Line < matches[j].Line
		}
		return matches[i].Path < matches[j].Path
	})
	return matches, nil
}

func grepFile(path, pattern string) ([]traceMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make([]traceMatch, 0, 4)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if strings.Contains(line, pattern) {
			out = append(out, traceMatch{
				Path: filepath.ToSlash(path),
				Line: lineNo,
				Text: line,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
