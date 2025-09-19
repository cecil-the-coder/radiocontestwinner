package transcriber

import "fmt"

// TranscriptionSegment represents a single, raw transcribed segment of audio as output by the Whisper.cpp model
type TranscriptionSegment struct {
	Text       string  `json:"text"`
	StartMS    int     `json:"start_ms"`
	EndMS      int     `json:"end_ms"`
	Confidence float32 `json:"confidence"`
}

// Validate checks if the TranscriptionSegment has valid values
func (ts *TranscriptionSegment) Validate() error {
	if ts.Text == "" {
		return fmt.Errorf("text cannot be empty")
	}

	if ts.StartMS < 0 {
		return fmt.Errorf("start_ms cannot be negative")
	}

	if ts.EndMS <= ts.StartMS {
		return fmt.Errorf("end_ms must be greater than start_ms")
	}

	if ts.Confidence < 0.0 || ts.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0")
	}

	return nil
}
