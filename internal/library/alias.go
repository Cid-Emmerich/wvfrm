package library

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Aliases maps artist names that should be shown as another artist. It is
// what makes a merge stick for files whose tags cannot be rewritten (and
// makes the library regroup instantly, before any file is touched).
//
// File format (~/.config/wvfrm/aliases): one "old name = new name" per line.
type Aliases struct {
	Path string
	m    map[string]string // norm(old) -> new
}

// LoadAliases reads the alias file; a missing file is an empty set.
func LoadAliases(path string) *Aliases {
	a := &Aliases{Path: path, m: map[string]string{}}
	f, err := os.Open(path)
	if err != nil {
		return a
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		old, new, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(old) != "" && strings.TrimSpace(new) != "" {
			a.m[norm(old)] = strings.TrimSpace(new)
		}
	}
	return a
}

// Resolve returns the display name for an artist, following aliases.
func (a *Aliases) Resolve(name string) string {
	if a == nil {
		return name
	}
	for i := 0; i < 8; i++ { // follow chains, but never loop forever
		to, ok := a.m[norm(name)]
		if !ok || norm(to) == norm(name) {
			return name
		}
		name = to
	}
	return name
}

// Add records old -> new and rewrites the file.
func (a *Aliases) Add(old, new string) error {
	if a.m == nil {
		a.m = map[string]string{}
	}
	a.m[norm(old)] = strings.TrimSpace(new)
	// anything that pointed at old now points at new
	for k, v := range a.m {
		if norm(v) == norm(old) {
			a.m[k] = strings.TrimSpace(new)
		}
	}
	return a.save()
}

// Len reports how many aliases are defined.
func (a *Aliases) Len() int { return len(a.m) }

func (a *Aliases) save() error {
	if a.Path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(a.Path), 0o755); err != nil {
		return err
	}
	keys := make([]string, 0, len(a.m))
	for k := range a.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("# wvfrm artist aliases: shown-as mappings created by the merge tool (M in the library)\n")
	sb.WriteString("# old name = new name\n")
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s = %s\n", k, a.m[k])
	}
	return os.WriteFile(a.Path, []byte(sb.String()), 0o644)
}
