package parser

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"sort"
	"time"
)

// ContestCue represents a successfully identified and validated contest cue
type ContestCue struct {
	CueID       string                 `json:"cue_id"`
	ContestType string                 `json:"contest_type"`
	Timestamp   string                 `json:"timestamp"`
	Details     map[string]interface{} `json:"details"`
}

// NewContestCue creates a new ContestCue with generated CueID and current timestamp
func NewContestCue(contestType string, details map[string]interface{}) *ContestCue {
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Add some randomness for uniqueness while maintaining deterministic deduplication
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomSeed := int64(randomBytes[0])<<56 | int64(randomBytes[1])<<48 | int64(randomBytes[2])<<40 | int64(randomBytes[3])<<32 |
		int64(randomBytes[4])<<24 | int64(randomBytes[5])<<16 | int64(randomBytes[6])<<8 | int64(randomBytes[7])
	cueID := GenerateCueIDWithSeed(contestType, details, timestamp, randomSeed)

	return &ContestCue{
		CueID:       cueID,
		ContestType: contestType,
		Timestamp:   timestamp,
		Details:     details,
	}
}

// GenerateCueID generates a unique CueID hash for deduplication
func GenerateCueID(contestType string, details map[string]interface{}, timestamp string) string {
	// Create a deterministic hash based on contestType, details, and timestamp
	h := sha256.New()
	h.Write([]byte(contestType))
	h.Write([]byte(timestamp))

	// Add details to hash in a deterministic way by sorting keys
	if details != nil {
		keys := make([]string, 0, len(details))
		for key := range details {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			h.Write([]byte(key))
			h.Write([]byte(fmt.Sprintf("%v", details[key])))
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil))[:16] // Use first 16 characters for readability
}

// GenerateCueIDWithSeed generates a unique CueID hash with optional random seed
func GenerateCueIDWithSeed(contestType string, details map[string]interface{}, timestamp string, seed int64) string {
	// Create a deterministic hash based on contestType, details, timestamp, and seed
	h := sha256.New()
	h.Write([]byte(contestType))
	h.Write([]byte(timestamp))
	h.Write([]byte(fmt.Sprintf("%d", seed)))

	// Add details to hash in a deterministic way by sorting keys
	if details != nil {
		keys := make([]string, 0, len(details))
		for key := range details {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			h.Write([]byte(key))
			h.Write([]byte(fmt.Sprintf("%v", details[key])))
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil))[:16] // Use first 16 characters for readability
}

// Validate checks if the ContestCue has valid values
func (cc *ContestCue) Validate() error {
	if cc.CueID == "" {
		return fmt.Errorf("CueID cannot be empty")
	}

	if cc.ContestType == "" {
		return fmt.Errorf("ContestType cannot be empty")
	}

	if cc.Timestamp == "" {
		return fmt.Errorf("Timestamp cannot be empty")
	}

	if cc.Details == nil {
		return fmt.Errorf("Details cannot be nil")
	}

	return nil
}
