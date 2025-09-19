package buffer

import "fmt"

// BufferedContext represents a collection of TranscriptionSegments that have been
// combined to form a more coherent sentence or phrase for easier parsing
type BufferedContext struct {
	Text    string `json:"text"`
	StartMS int    `json:"start_ms"`
	EndMS   int    `json:"end_ms"`
}

// Validate checks if the BufferedContext has valid values
func (bc *BufferedContext) Validate() error {
	if bc.Text == "" {
		return fmt.Errorf("text cannot be empty")
	}

	if bc.StartMS < 0 {
		return fmt.Errorf("start_ms cannot be negative")
	}

	if bc.EndMS <= bc.StartMS {
		return fmt.Errorf("end_ms must be greater than start_ms")
	}

	return nil
}