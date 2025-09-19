package parser

import (
	"regexp"

	"radiocontestwinner/internal/buffer"
)

// ContestParser filters BufferedContext based on number allowlist
type ContestParser struct {
	allowlist []string
}

// NewContestParser creates a new ContestParser with the given allowlist
func NewContestParser(allowlist []string) *ContestParser {
	return &ContestParser{
		allowlist: allowlist,
	}
}

// FilterByAllowlist checks if the BufferedContext contains any number from the allowlist
func (cp *ContestParser) FilterByAllowlist(context *buffer.BufferedContext) bool {
	if context == nil || cp.allowlist == nil || len(cp.allowlist) == 0 {
		return false
	}

	if context.Text == "" {
		return false
	}

	// Extract numbers from the text
	numbers := cp.ExtractNumbers(context.Text)

	// Check if any extracted number matches allowlist
	for _, extractedNum := range numbers {
		for _, allowedNum := range cp.allowlist {
			if extractedNum == allowedNum {
				return true
			}
		}
	}

	return false
}

// ExtractNumbers extracts all numbers from the given text
func (cp *ContestParser) ExtractNumbers(text string) []string {
	if text == "" {
		return []string{}
	}

	// Regular expression to match numbers (including those with leading zeros)
	numberRegex := regexp.MustCompile(`\d+`)
	matches := numberRegex.FindAllString(text, -1)

	return matches
}

// ProcessBufferedContext processes a stream of BufferedContext, filtering by allowlist
func (cp *ContestParser) ProcessBufferedContext(inputCh <-chan buffer.BufferedContext, outputCh chan<- buffer.BufferedContext) {
	defer close(outputCh)

	for context := range inputCh {
		if cp.FilterByAllowlist(&context) {
			select {
			case outputCh <- context:
				// Successfully sent
			default:
				// Output channel full, skip this context
			}
		}
	}
}