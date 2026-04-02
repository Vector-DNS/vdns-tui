package history

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Vector-DNS/vdns-tui/internal/config"
)

const (
	defaultFileName = "history.jsonl"
	filePerms       = 0600
	dirPerms        = 0700
)

// Entry represents a single history record.
type Entry struct {
	Timestamp  time.Time `json:"timestamp"`
	Command    string    `json:"command"`
	Domain     string    `json:"domain"`
	RecordType string    `json:"record_type,omitempty"`
	Mode       string    `json:"mode"`
	Results    any       `json:"results,omitempty"`
}

// filePath returns the history file path, checking the config override first.
func filePath(cfg *config.Config) (string, error) {
	if cfg != nil && cfg.Local.HistoryFile != "" {
		return cfg.Local.HistoryFile, nil
	}
	dataDir, err := config.DataDir()
	if err != nil {
		return "", fmt.Errorf("could not determine data directory: %w", err)
	}
	return filepath.Join(dataDir, defaultFileName), nil
}

// Append writes a single entry to the history file.
func Append(cfg *config.Config, entry Entry) error {
	path, err := filePath(cfg)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerms); err != nil {
		return fmt.Errorf("could not create data directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerms)
	if err != nil {
		return fmt.Errorf("could not open history file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("could not serialize entry: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("could not write entry: %w", err)
	}

	// Trim the file if it exceeds max entries.
	if cfg.Local.MaxHistoryItems > 0 {
		_ = trimIfNeeded(cfg)
	}

	return nil
}

// trimIfNeeded reads the history file and rewrites it with only the most recent entries
// if it exceeds the configured max.
func trimIfNeeded(cfg *config.Config) error {
	entries, err := readAll(cfg)
	if err != nil || len(entries) <= cfg.Local.MaxHistoryItems {
		return err
	}

	entries = entries[len(entries)-cfg.Local.MaxHistoryItems:]

	path, err := filePath(cfg)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, filePerms)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		f.Write(append(data, '\n'))
	}
	return nil
}

// readAll reads all entries from the history file.
func readAll(cfg *config.Config) ([]Entry, error) {
	path, err := filePath(cfg)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("could not open history file: %w", err)
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	// Allow lines up to 1MB for large result sets.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			// Skip malformed lines rather than failing entirely.
			continue
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("could not read history file: %w", err)
	}
	return entries, nil
}

// List returns the last N entries from the history file.
func List(cfg *config.Config, limit int) ([]Entry, error) {
	entries, err := readAll(cfg)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	return entries, nil
}

// Search filters entries by domain (case-insensitive exact match).
func Search(cfg *config.Config, domain string) ([]Entry, error) {
	entries, err := readAll(cfg)
	if err != nil {
		return nil, err
	}

	domain = strings.ToLower(domain)
	var matched []Entry
	for _, e := range entries {
		if strings.ToLower(e.Domain) == domain {
			matched = append(matched, e)
		}
	}
	return matched, nil
}

// Clear truncates the history file.
func Clear(cfg *config.Config) error {
	path, err := filePath(cfg)
	if err != nil {
		return err
	}

	if err := os.Truncate(path, 0); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("could not clear history: %w", err)
	}
	return nil
}

// Export writes history entries to w in the given format ("json" or "csv").
func Export(cfg *config.Config, format string, w io.Writer) error {
	entries, err := readAll(cfg)
	if err != nil {
		return err
	}

	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	case "csv":
		cw := csv.NewWriter(w)
		defer cw.Flush()

		if err := cw.Write([]string{"timestamp", "command", "domain", "record_type", "mode"}); err != nil {
			return err
		}
		for _, e := range entries {
			row := []string{
				e.Timestamp.Format(time.RFC3339),
				e.Command,
				e.Domain,
				e.RecordType,
				e.Mode,
			}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
		return cw.Error()
	default:
		return fmt.Errorf("unsupported export format: %s (use json or csv)", format)
	}
}
