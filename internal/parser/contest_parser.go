package parser

import (
	"fmt"
	"regexp"

	"go.uber.org/zap"

	"radiocontestwinner/internal/buffer"
)

// ContestParser filters BufferedContext based on number allowlist
type ContestParser struct {
	allowlist []string
	logger    *zap.Logger
}

// NewContestParser creates a new ContestParser with the given allowlist
func NewContestParser(allowlist []string) *ContestParser {
	return &ContestParser{
		allowlist: allowlist,
		logger:    zap.NewNop(), // Default to no-op logger
	}
}

// NewContestParserWithLogger creates a new ContestParser with the given allowlist and logger
func NewContestParserWithLogger(allowlist []string, logger *zap.Logger) *ContestParser {
	if logger == nil {
		logger = zap.NewNop() // Use no-op logger if nil is passed
	}
	return &ContestParser{
		allowlist: allowlist,
		logger:    logger,
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

// MatchContestPattern matches the "Text [KEYWORD] to [NUMBER]" pattern in the given text
// Returns keyword, number, and whether a valid match was found
func (cp *ContestParser) MatchContestPattern(text string) (keyword, number string, matched bool) {
	// Log the pattern matching attempt
	cp.logger.Debug("attempting pattern matching",
		zap.String("text", text),
		zap.Int("allowlist_size", len(cp.allowlist)))

	if text == "" || cp.allowlist == nil || len(cp.allowlist) == 0 {
		cp.logger.Debug("pattern matching failed - empty text or allowlist",
			zap.Bool("empty_text", text == ""),
			zap.Bool("nil_allowlist", cp.allowlist == nil),
			zap.Int("allowlist_size", len(cp.allowlist)))
		return "", "", false
	}

	// Create regex pattern for "Text [KEYWORD] to [NUMBER]"
	// Case-insensitive matching for "Text" and "to", but preserve case for keyword
	pattern := `(?i)\btext\s+(\S+)\s+to\s+(\d+)\b`
	regex := regexp.MustCompile(pattern)

	matches := regex.FindStringSubmatch(text)
	if len(matches) < 3 {
		cp.logger.Debug("pattern matching failed - no regex match",
			zap.String("pattern", pattern),
			zap.String("text", text))
		return "", "", false
	}

	extractedKeyword := matches[1]
	extractedNumber := matches[2]

	cp.logger.Debug("pattern regex matched",
		zap.String("keyword", extractedKeyword),
		zap.String("number", extractedNumber))

	// Validate extracted number against allowlist
	for _, allowedNum := range cp.allowlist {
		if extractedNumber == allowedNum {
			cp.logger.Info("pattern matching successful",
				zap.String("keyword", extractedKeyword),
				zap.String("number", extractedNumber),
				zap.String("text", text))
			return extractedKeyword, extractedNumber, true
		}
	}

	cp.logger.Debug("pattern matching failed - number not in allowlist",
		zap.String("keyword", extractedKeyword),
		zap.String("number", extractedNumber),
		zap.Strings("allowlist", cp.allowlist))

	return "", "", false
}

// CreateContestCue creates a ContestCue from BufferedContext if pattern matches
// Returns the ContestCue and whether it was successfully created
func (cp *ContestParser) CreateContestCue(context *buffer.BufferedContext) (*ContestCue, bool) {
	if context == nil {
		cp.logger.Warn("attempted to create ContestCue from nil context")
		return nil, false
	}

	cp.logger.Debug("creating ContestCue from context",
		zap.String("text", context.Text),
		zap.Int("start_ms", context.StartMS),
		zap.Int("end_ms", context.EndMS))

	// Try to match the contest pattern
	keyword, number, matched := cp.MatchContestPattern(context.Text)
	if !matched {
		cp.logger.Debug("ContestCue creation failed - no pattern match",
			zap.String("text", context.Text))
		return nil, false
	}

	// Create details map with extracted information
	details := map[string]interface{}{
		"keyword":       keyword,
		"number":        number,
		"original_text": context.Text,
		"start_ms":      context.StartMS,
		"end_ms":        context.EndMS,
	}

	// Create ContestCue with the keyword as the contest type
	cue := NewContestCue(keyword, details)

	// Validate the created cue
	if err := cue.Validate(); err != nil {
		cp.logger.Error("ContestCue validation failed",
			zap.Error(fmt.Errorf("failed to validate ContestCue: %w", err)),
			zap.String("cue_id", cue.CueID),
			zap.String("contest_type", cue.ContestType))
		return nil, false
	}

	cp.logger.Info("ContestCue created successfully",
		zap.String("cue_id", cue.CueID),
		zap.String("contest_type", cue.ContestType),
		zap.String("keyword", keyword),
		zap.String("number", number))

	return cue, true
}

// ProcessBufferedContextWithPatternMatching processes BufferedContext stream and outputs ContestCue when patterns match
func (cp *ContestParser) ProcessBufferedContextWithPatternMatching(inputCh <-chan buffer.BufferedContext, outputCh chan<- ContestCue) {
	defer close(outputCh)

	cp.logger.Info("starting pattern matching processing pipeline")
	processedCount := 0
	successCount := 0

	for context := range inputCh {
		processedCount++
		cp.logger.Debug("processing buffered context",
			zap.Int("context_number", processedCount),
			zap.String("text", context.Text))

		// Try to create ContestCue from context (includes allowlist filtering and pattern matching)
		cue, created := cp.CreateContestCue(&context)
		if created {
			successCount++
			select {
			case outputCh <- *cue:
				cp.logger.Debug("ContestCue sent to output channel",
					zap.String("cue_id", cue.CueID),
					zap.Int("processed_count", processedCount),
					zap.Int("success_count", successCount))
			default:
				cp.logger.Warn("output channel full, skipping ContestCue",
					zap.String("cue_id", cue.CueID),
					zap.String("contest_type", cue.ContestType))
			}
		}
	}

	cp.logger.Info("pattern matching processing pipeline completed",
		zap.Int("total_processed", processedCount),
		zap.Int("successful_matches", successCount))
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