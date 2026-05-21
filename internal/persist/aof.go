package persist

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// AOFWriter appends commands to an append-only file.
type AOFWriter struct {
	file *os.File
	mu   sync.Mutex
}

// NewAOFWriter opens (or creates) an AOF file for appending.
func NewAOFWriter(path string) (*AOFWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open AOF file %s: %w", path, err)
	}
	return &AOFWriter{file: f}, nil
}

// Write appends a command line to the AOF.
func (w *AOFWriter) Write(cmdLine string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := fmt.Fprintln(w.file, cmdLine)
	return err
}

// Sync flushes the AOF to disk.
func (w *AOFWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

// Close closes the AOF file.
func (w *AOFWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// ReplayAOF reads commands from an AOF file line by line.
// It returns the raw command lines. Invalid/empty lines are skipped with a warning.
func ReplayAOF(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no AOF file — clean start
		}
		return nil, fmt.Errorf("failed to open AOF file %s: %w", path, err)
	}
	defer f.Close()

	var commands []string
	scanner := bufio.NewScanner(f)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		commands = append(commands, line)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("aof: warning: read error at line %d: %v (partial replay)", lineNo, err)
	}

	return commands, nil
}
