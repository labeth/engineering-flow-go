package engflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var reqIDPattern = regexp.MustCompile(`REQ-[A-Z0-9-]{2,}`)

func mustEnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeTextFile(path, data string) error {
	if err := mustEnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

func writeJSON(path string, v any) error {
	if err := mustEnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func safeFeatureName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "default"
	}
	var out strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	res := strings.Trim(out.String(), "-")
	if res == "" {
		return "default"
	}
	return res
}

func reqPrefixFromFeature(feature string) string {
	safe := safeFeatureName(feature)
	safe = strings.ToUpper(strings.ReplaceAll(safe, "-", "_"))
	safe = strings.ReplaceAll(safe, "_", "")
	if len(safe) > 12 {
		safe = safe[:12]
	}
	if safe == "" {
		safe = "FEATURE"
	}
	return safe
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		if _, ok := seen[it]; ok {
			continue
		}
		seen[it] = struct{}{}
		out = append(out, it)
	}
	return out
}

func parseRequirementIDs(text string) []string {
	ids := reqIDPattern.FindAllString(strings.ToUpper(text), -1)
	return uniqueStrings(ids)
}

func parseRequirementIDsFromFile(path string) ([]string, error) {
	text, err := readFile(path)
	if err != nil {
		return nil, err
	}
	ids := parseRequirementIDs(text)
	sort.Strings(ids)
	return ids, nil
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := mustEnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hashFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashBytes(b), nil
}

func hashDir(path string) (string, error) {
	type entry struct {
		rel  string
		hash string
	}
	entries := make([]entry, 0, 64)
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".engflow" {
				if p == path {
					return nil
				}
				return filepath.SkipDir
			}
			return nil
		}
		h, err := hashFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return err
		}
		entries = append(entries, entry{rel: filepath.ToSlash(rel), hash: h})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.rel)
		b.WriteByte(':')
		b.WriteString(e.hash)
		b.WriteByte('\n')
	}
	return hashBytes([]byte(b.String())), nil
}

func hashPath(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "<missing>", nil
		}
		return "", err
	}
	if st.IsDir() {
		return hashDir(path)
	}
	return hashFile(path)
}

func snapshotPaths(paths []string) (map[string]string, error) {
	out := make(map[string]string, len(paths))
	for _, p := range paths {
		h, err := hashPath(p)
		if err != nil {
			return nil, err
		}
		out[p] = h
	}
	return out, nil
}

func changedPaths(before, after map[string]string) []string {
	all := make([]string, 0, len(before)+len(after))
	seen := map[string]struct{}{}
	for k := range before {
		seen[k] = struct{}{}
	}
	for k := range after {
		seen[k] = struct{}{}
	}
	for k := range seen {
		all = append(all, k)
	}
	sort.Strings(all)
	out := make([]string, 0, len(all))
	for _, k := range all {
		if before[k] != after[k] {
			out = append(out, k)
		}
	}
	return out
}

func quoteYAML(s string) string {
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", " ")
	return fmt.Sprintf("\"%s\"", strings.TrimSpace(s))
}
