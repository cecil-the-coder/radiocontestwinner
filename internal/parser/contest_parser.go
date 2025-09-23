package parser

import (
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"radiocontestwinner/internal/buffer"
)

// ContestParser filters BufferedContext based on number allowlist
type ContestParser struct {
	allowlist []string
	logger    *zap.Logger
	// Pre-compiled regexes for performance
	punctuationRegex *regexp.Regexp
	letterRegex      *regexp.Regexp
}

// NewContestParser creates a new ContestParser with the given allowlist
func NewContestParser(allowlist []string) *ContestParser {
	return &ContestParser{
		allowlist:        allowlist,
		logger:           zap.NewNop(), // Default to no-op logger
		punctuationRegex: regexp.MustCompile(`[^\w]`),
		letterRegex:      regexp.MustCompile(`[A-Za-z]`),
	}
}

// NewContestParserWithLogger creates a new ContestParser with the given allowlist and logger
func NewContestParserWithLogger(allowlist []string, logger *zap.Logger) *ContestParser {
	if logger == nil {
		logger = zap.NewNop() // Use no-op logger if nil is passed
	}
	return &ContestParser{
		allowlist:        allowlist,
		logger:           logger,
		punctuationRegex: regexp.MustCompile(`[^\w]`),
		letterRegex:      regexp.MustCompile(`[A-Za-z]`),
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
// NOTE: This function also applies spelled word reconstruction before pattern matching
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

	// Apply spelled-out word reconstruction before pattern matching
	originalText := text
	reconstructedText := cp.ReconstructSpelledWords(originalText)

	if reconstructedText != originalText {
		cp.logger.Debug("applied spelled word reconstruction in MatchContestPattern",
			zap.String("original_text", originalText),
			zap.String("reconstructed_text", reconstructedText))
	}

	// Create regex pattern for "Text [KEYWORD] to [NUMBER]"
	// Case-insensitive matching for "Text" and "to", but preserve case for keyword
	pattern := `(?i)\btext\s+(\S+)\s+to\s+(\d+)\b`
	regex := regexp.MustCompile(pattern)

	matches := regex.FindStringSubmatch(reconstructedText)
	if len(matches) < 3 {
		cp.logger.Debug("pattern matching failed - no regex match",
			zap.String("pattern", pattern),
			zap.String("original_text", originalText),
			zap.String("reconstructed_text", reconstructedText))
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
				zap.String("original_text", originalText),
				zap.String("reconstructed_text", reconstructedText))
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

	// Apply spelled-out word reconstruction before pattern matching
	originalText := context.Text
	reconstructedText := cp.ReconstructSpelledWords(originalText)

	if reconstructedText != originalText {
		cp.logger.Debug("applied spelled word reconstruction",
			zap.String("original_text", originalText),
			zap.String("reconstructed_text", reconstructedText))
	}

	// Try to match the contest pattern on reconstructed text
	keyword, number, matched := cp.MatchContestPattern(reconstructedText)
	if !matched {
		cp.logger.Debug("ContestCue creation failed - no pattern match",
			zap.String("original_text", originalText),
			zap.String("reconstructed_text", reconstructedText))
		return nil, false
	}

	// Create details map with extracted information
	details := map[string]interface{}{
		"keyword":            keyword,
		"number":             number,
		"original_text":      originalText,
		"reconstructed_text": reconstructedText,
		"start_ms":           context.StartMS,
		"end_ms":             context.EndMS,
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

// DetectLetterSequences identifies consecutive single letters in text that could be spelled-out words
// Returns slice of normalized letter sequences (minimum 3 letters)
func (cp *ContestParser) DetectLetterSequences(text string) []string {
	if text == "" {
		return []string{}
	}

	cp.logger.Debug("detecting letter sequences in text",
		zap.String("text", text))

	// Scan word by word to find sequences of single letters
	words := strings.Fields(text)
	var currentSequence []string
	var sequences []string

	for _, word := range words {
		// Clean word from punctuation
		cleanWord := cp.punctuationRegex.ReplaceAllString(word, "")

		// Check if it's a single letter
		if len(cleanWord) == 1 && cp.letterRegex.MatchString(cleanWord) {
			currentSequence = append(currentSequence, cleanWord)
		} else {
			// Not a single letter, check if we have a valid sequence
			if len(currentSequence) >= 3 {
				sequence := strings.Join(currentSequence, " ")
				sequences = append(sequences, sequence)
				cp.logger.Debug("detected letter sequence",
					zap.String("sequence", sequence),
					zap.Int("length", len(currentSequence)))
			}
			currentSequence = []string{}
		}
	}

	// Check final sequence
	if len(currentSequence) >= 3 {
		sequence := strings.Join(currentSequence, " ")
		sequences = append(sequences, sequence)
		cp.logger.Debug("detected final letter sequence",
			zap.String("sequence", sequence),
			zap.Int("length", len(currentSequence)))
	}

	cp.logger.Debug("completed letter sequence detection",
		zap.Int("total_sequences", len(sequences)))

	return sequences
}

// detectHyphenSequences finds hyphen-separated letter sequences in the text
// Returns slice of space-separated sequences for consistency with DetectLetterSequences
func (cp *ContestParser) detectHyphenSequences(text string) []string {
	var sequences []string

	// Split text into words and check each word for hyphen-separated letters
	words := strings.Fields(text)
	for _, word := range words {
		sequence := cp.detectHyphenatedSequence(word)
		if sequence != "" {
			sequences = append(sequences, sequence)
		}
	}

	return sequences
}

// detectHyphenatedSequence checks if a word contains hyphen-separated single letters
// Returns the sequence in space-separated format, or empty string if not a valid sequence
func (cp *ContestParser) detectHyphenatedSequence(word string) string {
	// Check if the word contains hyphens
	if !strings.Contains(word, "-") {
		return ""
	}

	// Split by hyphens
	parts := strings.Split(word, "-")
	if len(parts) < 3 {
		// Need at least 3 letters for a valid sequence
		return ""
	}

	var letters []string
	for _, part := range parts {
		// Clean each part from punctuation and check if it's a single letter
		cleanPart := cp.punctuationRegex.ReplaceAllString(part, "")
		if len(cleanPart) == 1 && cp.letterRegex.MatchString(cleanPart) {
			letters = append(letters, cleanPart)
		} else {
			// Not a single letter, this is not a valid hyphenated sequence
			return ""
		}
	}

	// Only return if we have at least 3 valid letters
	if len(letters) >= 3 {
		return strings.Join(letters, " ")
	}

	return ""
}

// ReconstructWord combines a letter sequence into a single word with proper case normalization
func (cp *ContestParser) ReconstructWord(sequence string) string {
	if sequence == "" {
		return ""
	}

	cp.logger.Debug("reconstructing word from sequence",
		zap.String("sequence", sequence))

	// Split by whitespace and extract letters
	parts := strings.Fields(sequence)
	var letters []string

	for _, part := range parts {
		// Remove any punctuation and get just the letter
		cleanPart := cp.punctuationRegex.ReplaceAllString(part, "")
		if len(cleanPart) == 1 && cp.letterRegex.MatchString(cleanPart) {
			letters = append(letters, strings.ToUpper(cleanPart))
		}
	}

	result := strings.Join(letters, "")

	cp.logger.Debug("reconstructed word",
		zap.String("sequence", sequence),
		zap.String("result", result))

	return result
}

// ReconstructSpelledWords processes text to find and replace spelled-out letter sequences with reconstructed words
func (cp *ContestParser) ReconstructSpelledWords(text string) string {
	if text == "" {
		return text
	}

	cp.logger.Debug("reconstructing spelled words in text",
		zap.String("original_text", text))

	// Find all letter sequences (both space-separated and hyphen-separated)
	spaceSequences := cp.DetectLetterSequences(text)
	hyphenSequences := cp.detectHyphenSequences(text)

	// Combine both types of sequences
	sequences := append(spaceSequences, hyphenSequences...)

	if len(sequences) == 0 {
		cp.logger.Debug("no letter sequences found for reconstruction")
		return text
	}

	result := text

	// Replace each sequence with reconstructed word
	for _, sequence := range sequences {
		word := cp.ReconstructWord(sequence)
		if word != "" {
			// Create pattern to match the sequence in text (handle various spacing/punctuation including hyphens)
			// Split the sequence into individual letters
			letters := strings.Fields(sequence)
			if len(letters) > 0 {
				// Build a pattern that matches the letters with flexible separators
				var patternParts []string
				for i, letter := range letters {
					// Escape each letter in case it has special regex meaning
					escapedLetter := regexp.QuoteMeta(letter)
					patternParts = append(patternParts, escapedLetter)

					// Add separator pattern between letters (but not after the last one)
					if i < len(letters)-1 {
						// Match spaces, hyphens, commas, and combinations thereof
						patternParts = append(patternParts, `\s*[-,\s]*\s*`)
					}
				}

				pattern := `\b` + strings.Join(patternParts, "") + `\b`

				regex := regexp.MustCompile(pattern)
				if regex.MatchString(result) {
					result = regex.ReplaceAllString(result, word)
					cp.logger.Debug("replaced spelled sequence with word",
						zap.String("sequence", sequence),
						zap.String("word", word),
						zap.String("pattern", pattern))
				}
			}
		}
	}

	cp.logger.Debug("completed spelled word reconstruction",
		zap.String("original_text", text),
		zap.String("reconstructed_text", result))

	return result
}
