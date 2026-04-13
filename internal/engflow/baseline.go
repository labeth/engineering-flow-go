package engflow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type scaffoldBaseline struct {
	CreatedAt       string            `json:"created_at"`
	CanonicalHashes map[string]string `json:"canonical_hashes"`
}

func baselineStatePath(repoRoot string) string {
	return filepath.Join(repoRoot, ".engflow", "state", "scaffold-baseline.json")
}

func writeScaffoldBaseline(repoRoot string, cfg runConfig, force bool) error {
	path := baselineStatePath(repoRoot)
	if !force && fileExists(path) {
		return nil
	}
	canonicalPaths := canonicalInputPaths(cfg)
	hashes, err := snapshotCanonicalHashes(repoRoot, canonicalPaths)
	if err != nil {
		return err
	}
	payload := scaffoldBaseline{
		CreatedAt:       nowRFC3339(),
		CanonicalHashes: hashes,
	}
	return writeJSON(path, payload)
}

func loadScaffoldBaseline(repoRoot string) (scaffoldBaseline, error) {
	path := baselineStatePath(repoRoot)
	b, err := os.ReadFile(path)
	if err != nil {
		return scaffoldBaseline{}, err
	}
	var payload scaffoldBaseline
	if err := json.Unmarshal(b, &payload); err != nil {
		return scaffoldBaseline{}, err
	}
	if strings.TrimSpace(payload.CreatedAt) == "" || len(payload.CanonicalHashes) == 0 {
		return scaffoldBaseline{}, errors.New("invalid baseline payload")
	}
	return payload, nil
}

func canonicalInputPaths(cfg runConfig) []string {
	out := []string{
		filepath.ToSlash(filepath.Clean(strings.TrimSpace(cfg.CatalogPath))),
		filepath.ToSlash(filepath.Clean(strings.TrimSpace(cfg.RequirementsPath))),
		filepath.ToSlash(filepath.Clean(strings.TrimSpace(cfg.DesignPath))),
		filepath.ToSlash(filepath.Clean(strings.TrimSpace(cfg.ArchitecturePath))),
	}
	return uniqueStrings(out)
}

func canonicalHashesChanged(before, after map[string]string) bool {
	seen := map[string]struct{}{}
	for k := range before {
		seen[k] = struct{}{}
	}
	for k := range after {
		seen[k] = struct{}{}
	}
	for k := range seen {
		if before[k] != after[k] {
			return true
		}
	}
	return false
}

func collectImplementationFilesChangedSince(repoRoot string, since time.Time, canonicalPaths []string) ([]string, error) {
	canonical := map[string]struct{}{}
	for _, p := range canonicalPaths {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
		if clean == "" || clean == "." {
			continue
		}
		canonical[clean] = struct{}{}
	}
	canonical[filepath.ToSlash(filepath.Clean("ARCHITECTURE.ai.json"))] = struct{}{}
	canonical[filepath.ToSlash(filepath.Clean("spec.md"))] = struct{}{}

	isCodeExt := map[string]bool{
		".go": true, ".rs": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".java": true, ".kt": true, ".cs": true, ".c": true, ".cc": true, ".cpp": true,
		".h": true, ".hpp": true, ".swift": true, ".rb": true, ".php": true, ".py": true,
		".sql": true, ".tf": true, ".hcl": true,
	}
	out := make([]string, 0, 16)
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == ".engflow" || name == ".specify" || name == ".opencode" || name == ".github" || name == "node_modules" || name == "vendor" {
				if filepath.Clean(path) == filepath.Clean(repoRoot) {
					return nil
				}
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		if _, skip := canonical[rel]; skip {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if !isCodeExt[ext] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(since) {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return uniqueStrings(out), nil
}

func snapshotCanonicalHashes(repoRoot string, canonicalPaths []string) (map[string]string, error) {
	out := make(map[string]string, len(canonicalPaths))
	for _, rel := range canonicalPaths {
		rel = filepath.ToSlash(filepath.Clean(strings.TrimSpace(rel)))
		if rel == "" || rel == "." {
			continue
		}
		path := rel
		if !filepath.IsAbs(path) {
			path = filepath.Join(repoRoot, filepath.FromSlash(rel))
		}
		h, err := hashPath(path)
		if err != nil {
			return nil, err
		}
		out[rel] = h
	}
	return out, nil
}
