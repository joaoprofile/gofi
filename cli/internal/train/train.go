// Package train installs and manages markdown knowledge files that gofi
// agents read before each interaction. Files land under
// .claude/knowledge/{shared,pd,spec,eng,qa}/<topic>.md with a small header
// recording where the content came from.
package train

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ScopeShared = "shared"

	HeaderStart = "<!-- gofi-train"
	HeaderEnd   = "-->"
)

// slugRe accepts a leading lowercase letter, then optional lowercase letters,
// digits and hyphens, ending with a letter or digit (no trailing hyphen).
// Single-char slugs ("x") are allowed.
var slugRe = regexp.MustCompile(`^[a-z]([a-z0-9-]*[a-z0-9])?$`)

// AgentToScope maps full agent names to the scope name used for the
// knowledge subdirectory and config field. ScopeShared is used when --shared
// is set instead of an agent name.
var AgentToScope = map[string]string{
	"gofi-pd":   "pd",
	"gofi-spec": "spec",
	"gofi-eng":  "eng",
	"gofi-qa":   "qa",
}

// Item is the metadata persisted in .gofi.yaml after a successful install.
type Item struct {
	Topic       string
	Source      string
	InstalledAt string
	Hash        string
}

// ValidateTopic enforces the slug shape used both as filename and yaml key.
func ValidateTopic(topic string) error {
	if !slugRe.MatchString(topic) {
		return fmt.Errorf("topic %q must match ^[a-z][a-z0-9-]+$", topic)
	}
	return nil
}

// ScopeDir resolves the knowledge directory for the given scope (e.g. "pd",
// "shared"). The directory is *not* created here.
func ScopeDir(projectRoot, scope string) string {
	return filepath.Join(projectRoot, ".claude", "knowledge", scope)
}

// TopicPath returns the on-disk path for a topic in scope.
func TopicPath(projectRoot, scope, topic string) string {
	return filepath.Join(ScopeDir(projectRoot, scope), topic+".md")
}

// Install writes content (markdown bytes provided by the caller) into the
// knowledge dir, prefixed with a gofi-train header. The returned Item is
// ready to be persisted into .gofi.yaml.
//
// Empty content is rejected. When the topic file already exists, replace must
// be true; otherwise an error is returned.
func Install(projectRoot, scope, topic, sourceLabel string, content []byte, today string, replace bool) (Item, error) {
	if err := ValidateTopic(topic); err != nil {
		return Item{}, err
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return Item{}, errors.New("content is empty")
	}

	dir := ScopeDir(projectRoot, scope)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Item{}, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	target := TopicPath(projectRoot, scope, topic)
	if !replace {
		if _, err := os.Stat(target); err == nil {
			return Item{}, fmt.Errorf("topic %q already exists in %s; pass --replace to overwrite", topic, scope)
		}
	}

	hash := hashContent(content)
	header := buildHeader(sourceLabel, today, hash)
	full := []byte(header + string(content))
	if !strings.HasSuffix(string(content), "\n") {
		full = append(full, '\n')
	}

	if err := os.WriteFile(target, full, 0o644); err != nil {
		return Item{}, fmt.Errorf("write %s: %w", target, err)
	}

	return Item{
		Topic:       topic,
		Source:      sourceLabel,
		InstalledAt: today,
		Hash:        "sha256:" + hash,
	}, nil
}

// Read returns the on-disk content of the topic file (header included).
func Read(projectRoot, scope, topic string) ([]byte, error) {
	return os.ReadFile(TopicPath(projectRoot, scope, topic))
}

// Remove deletes the topic file. Missing files are not an error.
func Remove(projectRoot, scope, topic string) error {
	target := TopicPath(projectRoot, scope, topic)
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// List returns the topic names found on disk for the scope, sorted.
func List(projectRoot, scope string) ([]string, error) {
	dir := ScopeDir(projectRoot, scope)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var topics []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".md") {
			topics = append(topics, strings.TrimSuffix(name, ".md"))
		}
	}
	return topics, nil
}

func hashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func buildHeader(source, installedAt, hash string) string {
	var b strings.Builder
	b.WriteString(HeaderStart + "\n")
	b.WriteString("  source: " + source + "\n")
	b.WriteString("  installed_at: " + installedAt + "\n")
	b.WriteString("  hash: sha256:" + hash + "\n")
	b.WriteString(HeaderEnd + "\n\n")
	return b.String()
}
