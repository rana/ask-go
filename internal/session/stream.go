package session

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// StreamWriter handles streaming writes to session.md
type StreamWriter struct {
	file          *os.File
	writer        *bufio.Writer
	turnNumber    int
	isInterrupted bool
}

// NewStreamWriter creates a new streaming writer for the AI response
func NewStreamWriter(path string, turnNumber int) (*StreamWriter, error) {
	// Open file for appending
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open session for writing: %w", err)
	}

	writer := bufio.NewWriter(file)

	// Write AI header with two blank lines before
	header := fmt.Sprintf("\n\n# [%d] AI\n\n````markdown\n", turnNumber)
	if _, err := writer.WriteString(header); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to write header: %w", err)
	}

	// Flush header immediately so it's visible
	if err := writer.Flush(); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to flush header: %w", err)
	}

	return &StreamWriter{
		file:       file,
		writer:     writer,
		turnNumber: turnNumber,
	}, nil
}

// WriteChunk writes a chunk of response content
func (sw *StreamWriter) WriteChunk(chunk string) error {
	if sw.isInterrupted {
		return nil // Don't write after interruption
	}

	if _, err := sw.writer.WriteString(chunk); err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	// Flush after each chunk for immediate visibility
	return sw.writer.Flush()
}

// Close finalizes the streaming session
func (sw *StreamWriter) Close(interrupted bool, tokenCount int) error {
	defer sw.file.Close()

	if interrupted && !sw.isInterrupted {
		sw.isInterrupted = true
		// Add interruption marker
		marker := fmt.Sprintf("\n[Interrupted after ~%d tokens]", tokenCount)
		sw.writer.WriteString(marker)
	}

	// Close markdown fence
	sw.writer.WriteString("\n````\n")

	// Add two blank lines and next Human turn
	nextTurn := fmt.Sprintf("\n\n# [%d] Human\n\n", sw.turnNumber+1)
	sw.writer.WriteString(nextTurn)

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
