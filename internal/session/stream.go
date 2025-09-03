package session

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// StreamWriter handles streaming writes to session.md
type StreamWriter struct {
	file           *os.File
	writer         *bufio.Writer
	turnNumber     int
	headerWritten  bool // Track if we've written the AI header
	contentWritten bool // Track if any actual content was written
	isInterrupted  bool
}

// NewStreamWriter creates a new streaming writer for the AI response
func NewStreamWriter(path string, turnNumber int) (*StreamWriter, error) {
	// Open file for appending
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open session for writing: %w", err)
	}

	return &StreamWriter{
		file:           file,
		writer:         bufio.NewWriter(file),
		turnNumber:     turnNumber,
		headerWritten:  false,
		contentWritten: false,
	}, nil
}

// writeHeader writes the AI header and markdown fence when first content arrives
func (sw *StreamWriter) writeHeader() error {
	if sw.headerWritten {
		return nil
	}

	header := fmt.Sprintf("\n\n# [%d] AI\n\n````markdown\n", sw.turnNumber)
	if _, err := sw.writer.WriteString(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Flush header immediately so it's visible
	if err := sw.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush header: %w", err)
	}

	sw.headerWritten = true
	return nil
}

// WriteChunk writes a chunk of response content
func (sw *StreamWriter) WriteChunk(chunk string) error {
	if sw.isInterrupted {
		return nil // Don't write after interruption
	}

	// Skip empty chunks
	if chunk == "" {
		return nil
	}

	// Write header on first real content
	if !sw.headerWritten {
		if err := sw.writeHeader(); err != nil {
			return err
		}
	}

	if _, err := sw.writer.WriteString(chunk); err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	sw.contentWritten = true

	// Flush after each chunk for immediate visibility
	return sw.writer.Flush()
}

// Close finalizes the streaming session
func (sw *StreamWriter) Close(interrupted bool, tokenCount int) error {
	defer sw.file.Close()

	// If nothing was written at all, just close and return
	if !sw.headerWritten {
		return nil
	}

	// Only write interruption marker if we actually started writing content
	if interrupted && !sw.isInterrupted && sw.contentWritten {
		sw.isInterrupted = true
		// Add interruption marker
		marker := fmt.Sprintf("\n[Interrupted after %d tokens]", tokenCount)
		sw.writer.WriteString(marker)
	}

	// Close markdown fence (only if we opened it)
	if sw.headerWritten {
		sw.writer.WriteString("\n````\n")

		// Only add next Human turn if we wrote AI content
		nextTurn := fmt.Sprintf("\n\n# [%d] Human\n\n", sw.turnNumber+1)
		sw.writer.WriteString(nextTurn)
	}

	// Final flush
	if err := sw.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush final content: %w", err)
	}

	// Sync to disk to ensure VSCode sees it
	return sw.file.Sync()
}

// StreamResponse handles the complete streaming response flow
func StreamResponse(path string, turnNumber int, streamFunc func(*StreamWriter) (int, error)) error {
	writer, err := NewStreamWriter(path, turnNumber)
	if err != nil {
		return err
	}

	tokenCount, streamErr := streamFunc(writer)

	// Determine if interrupted
	interrupted := streamErr != nil && strings.Contains(streamErr.Error(), "context canceled")

	// Always close properly
	if closeErr := writer.Close(interrupted, tokenCount); closeErr != nil {
		return fmt.Errorf("failed to close stream: %w", closeErr)
	}

	// Return stream error if not a cancellation
	if streamErr != nil && !interrupted {
		return streamErr
	}

	return nil
}
