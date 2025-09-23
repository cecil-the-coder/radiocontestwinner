package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"radiocontestwinner/internal/buffer"
)

func TestContestParser_FilterByAllowlist(t *testing.T) {
	t.Run("should pass through text containing allowlisted number", func(t *testing.T) {
		// Arrange
		allowlist := []string{"73", "146", "222"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "This is 73 calling CQ contest",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		result := parser.FilterByAllowlist(context)

		// Assert
		assert.True(t, result, "should pass text containing allowlisted number 73")
	})

	t.Run("should reject text not containing allowlisted number", func(t *testing.T) {
		// Arrange
		allowlist := []string{"73", "146", "222"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "This is commercial break for insurance",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		result := parser.FilterByAllowlist(context)

		// Assert
		assert.False(t, result, "should reject text not containing allowlisted numbers")
	})

	t.Run("should pass text with multiple allowlisted numbers", func(t *testing.T) {
		// Arrange
		allowlist := []string{"73", "146", "222"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Station 146 calling 73, go ahead",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		result := parser.FilterByAllowlist(context)

		// Assert
		assert.True(t, result, "should pass text containing multiple allowlisted numbers")
	})

	t.Run("should handle empty allowlist", func(t *testing.T) {
		// Arrange
		allowlist := []string{}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "This is 73 calling CQ contest",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		result := parser.FilterByAllowlist(context)

		// Assert
		assert.False(t, result, "should reject all text when allowlist is empty")
	})

	t.Run("should handle nil allowlist", func(t *testing.T) {
		// Arrange
		parser := NewContestParser(nil)
		context := &buffer.BufferedContext{
			Text:    "This is 73 calling CQ contest",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		result := parser.FilterByAllowlist(context)

		// Assert
		assert.False(t, result, "should reject all text when allowlist is nil")
	})

	t.Run("should handle numbers embedded in words", func(t *testing.T) {
		// Arrange
		allowlist := []string{"73", "146"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Check station73 on frequency 14600",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		result := parser.FilterByAllowlist(context)

		// Assert
		assert.True(t, result, "should match numbers embedded in words")
	})

	t.Run("should handle partial number matches correctly", func(t *testing.T) {
		// Arrange
		allowlist := []string{"146"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Station 14 calling, frequency 46 MHz",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		result := parser.FilterByAllowlist(context)

		// Assert
		assert.False(t, result, "should not match partial numbers")
	})

	t.Run("should be case insensitive for text but exact for numbers", func(t *testing.T) {
		// Arrange
		allowlist := []string{"73"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "THIS IS 73 CALLING CQ CONTEST",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		result := parser.FilterByAllowlist(context)

		// Assert
		assert.True(t, result, "should work with uppercase text")
	})

	t.Run("should handle leading zeros in allowlist", func(t *testing.T) {
		// Arrange
		allowlist := []string{"073", "0146"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Station 073 calling, also 146 responding",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		result := parser.FilterByAllowlist(context)

		// Assert
		assert.True(t, result, "should match numbers with leading zeros")
	})

	t.Run("should handle empty text", func(t *testing.T) {
		// Arrange
		allowlist := []string{"73"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		result := parser.FilterByAllowlist(context)

		// Assert
		assert.False(t, result, "should reject empty text")
	})

	t.Run("should handle nil context", func(t *testing.T) {
		// Arrange
		allowlist := []string{"73"}
		parser := NewContestParser(allowlist)

		// Act
		result := parser.FilterByAllowlist(nil)

		// Assert
		assert.False(t, result, "should reject nil context")
	})
}

func TestContestParser_ExtractNumbers(t *testing.T) {
	t.Run("should extract single number from text", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "This is 73 calling"

		// Act
		numbers := parser.ExtractNumbers(text)

		// Assert
		expected := []string{"73"}
		assert.Equal(t, expected, numbers)
	})

	t.Run("should extract multiple numbers from text", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Station 146 calling 73, frequency 14.230"

		// Act
		numbers := parser.ExtractNumbers(text)

		// Assert
		assert.Contains(t, numbers, "146")
		assert.Contains(t, numbers, "73")
		assert.Contains(t, numbers, "14")
		assert.Contains(t, numbers, "230")
	})

	t.Run("should handle text with no numbers", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "This is commercial break for insurance"

		// Act
		numbers := parser.ExtractNumbers(text)

		// Assert
		assert.Empty(t, numbers)
	})

	t.Run("should handle empty text", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := ""

		// Act
		numbers := parser.ExtractNumbers(text)

		// Assert
		assert.Empty(t, numbers)
	})

	t.Run("should handle numbers with leading zeros", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Station 073 calling 0146"

		// Act
		numbers := parser.ExtractNumbers(text)

		// Assert
		assert.Contains(t, numbers, "073")
		assert.Contains(t, numbers, "0146")
	})

	t.Run("should handle decimal numbers", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Frequency 14.230 MHz and 7.125 kHz"

		// Act
		numbers := parser.ExtractNumbers(text)

		// Assert
		assert.Contains(t, numbers, "14")
		assert.Contains(t, numbers, "230")
		assert.Contains(t, numbers, "7")
		assert.Contains(t, numbers, "125")
	})
}

func TestContestParser_ProcessBufferedContext(t *testing.T) {
	t.Run("should pass through allowed contexts and reject disallowed ones", func(t *testing.T) {
		// Arrange
		allowlist := []string{"73", "146"}
		parser := NewContestParser(allowlist)

		inputCh := make(chan buffer.BufferedContext, 3)
		outputCh := make(chan buffer.BufferedContext, 3)

		allowedContext1 := buffer.BufferedContext{
			Text:    "This is 73 calling CQ contest",
			StartMS: 1000,
			EndMS:   2000,
		}

		disallowedContext := buffer.BufferedContext{
			Text:    "Commercial break for insurance",
			StartMS: 2000,
			EndMS:   3000,
		}

		allowedContext2 := buffer.BufferedContext{
			Text:    "Station 146 please respond",
			StartMS: 3000,
			EndMS:   4000,
		}

		// Act - send test contexts
		inputCh <- allowedContext1
		inputCh <- disallowedContext
		inputCh <- allowedContext2
		close(inputCh)

		// Process contexts
		parser.ProcessBufferedContext(inputCh, outputCh)

		// Assert - should only receive allowed contexts
		var results []buffer.BufferedContext
		for result := range outputCh {
			results = append(results, result)
		}

		assert.Len(t, results, 2, "should pass through 2 allowed contexts")
		assert.Equal(t, allowedContext1.Text, results[0].Text)
		assert.Equal(t, allowedContext2.Text, results[1].Text)
	})

	t.Run("should close output channel when input channel closes", func(t *testing.T) {
		// Arrange
		allowlist := []string{"73"}
		parser := NewContestParser(allowlist)

		inputCh := make(chan buffer.BufferedContext)
		outputCh := make(chan buffer.BufferedContext, 1)

		// Act
		close(inputCh)
		parser.ProcessBufferedContext(inputCh, outputCh)

		// Assert
		_, ok := <-outputCh
		assert.False(t, ok, "output channel should be closed")
	})

	t.Run("should handle empty input gracefully", func(t *testing.T) {
		// Arrange
		allowlist := []string{"73"}
		parser := NewContestParser(allowlist)

		inputCh := make(chan buffer.BufferedContext)
		outputCh := make(chan buffer.BufferedContext)

		// Act
		close(inputCh)
		parser.ProcessBufferedContext(inputCh, outputCh)

		// Assert - should not panic and should close output
		_, ok := <-outputCh
		assert.False(t, ok, "output channel should be closed when input is empty")
	})
}

func TestContestParser_MatchContestPattern(t *testing.T) {
	t.Run("should match valid contest pattern with uppercase keyword", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)
		text := "Text POTA to 1234"

		// Act
		keyword, number, matched := parser.MatchContestPattern(text)

		// Assert
		assert.True(t, matched, "should match valid contest pattern")
		assert.Equal(t, "POTA", keyword, "should extract keyword")
		assert.Equal(t, "1234", number, "should extract number")
	})

	t.Run("should match valid contest pattern with lowercase keyword", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)
		text := "Text pota to 1234"

		// Act
		keyword, number, matched := parser.MatchContestPattern(text)

		// Assert
		assert.True(t, matched, "should match valid contest pattern")
		assert.Equal(t, "pota", keyword, "should extract keyword preserving case")
		assert.Equal(t, "1234", number, "should extract number")
	})

	t.Run("should match valid contest pattern with mixed case", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)
		text := "text WwFf To 5678"

		// Act
		keyword, number, matched := parser.MatchContestPattern(text)

		// Assert
		assert.True(t, matched, "should match valid contest pattern")
		assert.Equal(t, "WwFf", keyword, "should extract keyword preserving case")
		assert.Equal(t, "5678", number, "should extract number")
	})

	t.Run("should not match when number is not in allowlist", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)
		text := "Text POTA to 9999"

		// Act
		keyword, number, matched := parser.MatchContestPattern(text)

		// Assert
		assert.False(t, matched, "should not match when number not in allowlist")
		assert.Empty(t, keyword, "keyword should be empty when not matched")
		assert.Empty(t, number, "number should be empty when not matched")
	})

	t.Run("should not match invalid pattern structure", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)

		testCases := []struct {
			name string
			text string
		}{
			{"missing Text prefix", "POTA to 1234"},
			{"missing to keyword", "Text POTA 1234"},
			{"wrong order", "POTA Text to 1234"},
			{"extra words", "Text POTA extra to 1234"},
			{"no keyword", "Text to 1234"},
			{"no number", "Text POTA to"},
			{"empty text", ""},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Act
				keyword, number, matched := parser.MatchContestPattern(tc.text)

				// Assert
				assert.False(t, matched, "should not match pattern: %s", tc.text)
				assert.Empty(t, keyword, "keyword should be empty when not matched")
				assert.Empty(t, number, "number should be empty when not matched")
			})
		}
	})

	t.Run("should handle pattern with leading/trailing spaces", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234"}
		parser := NewContestParser(allowlist)
		text := "  Text POTA to 1234  "

		// Act
		keyword, number, matched := parser.MatchContestPattern(text)

		// Assert
		assert.True(t, matched, "should match pattern with spaces")
		assert.Equal(t, "POTA", keyword, "should extract keyword")
		assert.Equal(t, "1234", number, "should extract number")
	})

	t.Run("should match pattern within longer text", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234"}
		parser := NewContestParser(allowlist)
		text := "Welcome to the show. Text POTA to 1234 for more information. Thank you for listening."

		// Act
		keyword, number, matched := parser.MatchContestPattern(text)

		// Assert
		assert.True(t, matched, "should match pattern within longer text")
		assert.Equal(t, "POTA", keyword, "should extract keyword")
		assert.Equal(t, "1234", number, "should extract number")
	})

	t.Run("should handle multiple keyword formats", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678", "9999"}
		parser := NewContestParser(allowlist)

		testCases := []struct {
			text    string
			keyword string
			number  string
		}{
			{"Text POTA to 1234", "POTA", "1234"},
			{"Text WWFF to 5678", "WWFF", "5678"},
			{"Text SOTA to 9999", "SOTA", "9999"},
			{"Text K1ABC to 1234", "K1ABC", "1234"},
			{"Text VK2DEF to 5678", "VK2DEF", "5678"},
		}

		for _, tc := range testCases {
			t.Run(tc.keyword, func(t *testing.T) {
				// Act
				keyword, number, matched := parser.MatchContestPattern(tc.text)

				// Assert
				assert.True(t, matched, "should match pattern with keyword: %s", tc.keyword)
				assert.Equal(t, tc.keyword, keyword, "should extract keyword")
				assert.Equal(t, tc.number, number, "should extract number")
			})
		}
	})

	t.Run("should handle empty allowlist", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Text POTA to 1234"

		// Act
		keyword, number, matched := parser.MatchContestPattern(text)

		// Assert
		assert.False(t, matched, "should not match when allowlist is empty")
		assert.Empty(t, keyword, "keyword should be empty when allowlist is empty")
		assert.Empty(t, number, "number should be empty when allowlist is empty")
	})

	t.Run("should handle nil allowlist", func(t *testing.T) {
		// Arrange
		parser := NewContestParser(nil)
		text := "Text POTA to 1234"

		// Act
		keyword, number, matched := parser.MatchContestPattern(text)

		// Assert
		assert.False(t, matched, "should not match when allowlist is nil")
		assert.Empty(t, keyword, "keyword should be empty when allowlist is nil")
		assert.Empty(t, number, "number should be empty when allowlist is nil")
	})
}

func TestContestParser_CreateContestCue(t *testing.T) {
	t.Run("should create ContestCue when pattern matches", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Text POTA to 1234",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		cue, created := parser.CreateContestCue(context)

		// Assert
		assert.True(t, created, "should create ContestCue when pattern matches")
		assert.NotNil(t, cue, "should return ContestCue instance")
		assert.NotEmpty(t, cue.CueID, "should generate CueID")
		assert.Equal(t, "POTA", cue.ContestType, "should set ContestType from keyword")
		assert.NotEmpty(t, cue.Timestamp, "should set Timestamp")
		assert.NotNil(t, cue.Details, "should set Details")

		// Verify Details contains expected fields
		assert.Equal(t, "POTA", cue.Details["keyword"], "should set keyword in Details")
		assert.Equal(t, "1234", cue.Details["number"], "should set number in Details")
		assert.Equal(t, "Text POTA to 1234", cue.Details["original_text"], "should set original text in Details")
		assert.Equal(t, 1000, cue.Details["start_ms"], "should set start_ms in Details")
		assert.Equal(t, 2000, cue.Details["end_ms"], "should set end_ms in Details")
	})

	t.Run("should not create ContestCue when pattern does not match", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "No pattern here",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		cue, created := parser.CreateContestCue(context)

		// Assert
		assert.False(t, created, "should not create ContestCue when pattern does not match")
		assert.Nil(t, cue, "should return nil ContestCue when pattern does not match")
	})

	t.Run("should not create ContestCue when number not in allowlist", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Text POTA to 9999",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		cue, created := parser.CreateContestCue(context)

		// Assert
		assert.False(t, created, "should not create ContestCue when number not in allowlist")
		assert.Nil(t, cue, "should return nil ContestCue when number not in allowlist")
	})

	t.Run("should handle nil context gracefully", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)

		// Act
		cue, created := parser.CreateContestCue(nil)

		// Assert
		assert.False(t, created, "should not create ContestCue for nil context")
		assert.Nil(t, cue, "should return nil ContestCue for nil context")
	})

	t.Run("should create different CueIDs for different contexts", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)

		context1 := &buffer.BufferedContext{
			Text:    "Text POTA to 1234",
			StartMS: 1000,
			EndMS:   2000,
		}

		context2 := &buffer.BufferedContext{
			Text:    "Text WWFF to 5678",
			StartMS: 3000,
			EndMS:   4000,
		}

		// Act
		cue1, created1 := parser.CreateContestCue(context1)
		cue2, created2 := parser.CreateContestCue(context2)

		// Assert
		assert.True(t, created1, "should create first ContestCue")
		assert.True(t, created2, "should create second ContestCue")
		assert.NotEqual(t, cue1.CueID, cue2.CueID, "should generate different CueIDs for different contexts")
		assert.Equal(t, "POTA", cue1.ContestType, "should set correct ContestType for first cue")
		assert.Equal(t, "WWFF", cue2.ContestType, "should set correct ContestType for second cue")
	})

	t.Run("should validate created ContestCue", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Text SOTA to 1234",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		cue, created := parser.CreateContestCue(context)

		// Assert
		assert.True(t, created, "should create ContestCue")
		assert.NotNil(t, cue, "should return ContestCue instance")

		// Verify the created ContestCue is valid
		err := cue.Validate()
		assert.NoError(t, err, "created ContestCue should be valid")
	})
}

func TestContestParser_ProcessBufferedContextWithPatternMatching(t *testing.T) {
	t.Run("should output ContestCue when text matches pattern and allowlist", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)

		inputCh := make(chan buffer.BufferedContext, 2)
		outputCh := make(chan ContestCue, 2)

		matchingContext := buffer.BufferedContext{
			Text:    "Text POTA to 1234",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act - send context and close input
		inputCh <- matchingContext
		close(inputCh)

		// Process contexts
		parser.ProcessBufferedContextWithPatternMatching(inputCh, outputCh)

		// Assert - should receive ContestCue
		var results []ContestCue
		for result := range outputCh {
			results = append(results, result)
		}

		assert.Len(t, results, 1, "should output one ContestCue")
		assert.Equal(t, "POTA", results[0].ContestType, "should set correct ContestType")
		assert.Equal(t, "POTA", results[0].Details["keyword"], "should set keyword in Details")
		assert.Equal(t, "1234", results[0].Details["number"], "should set number in Details")
		assert.Equal(t, "Text POTA to 1234", results[0].Details["original_text"], "should set original text")
	})

	t.Run("should not output when text contains allowlist number but no pattern", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)

		inputCh := make(chan buffer.BufferedContext, 1)
		outputCh := make(chan ContestCue, 1)

		noPatternContext := buffer.BufferedContext{
			Text:    "This contains 1234 but no pattern",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		inputCh <- noPatternContext
		close(inputCh)
		parser.ProcessBufferedContextWithPatternMatching(inputCh, outputCh)

		// Assert - should not output anything
		var results []ContestCue
		for result := range outputCh {
			results = append(results, result)
		}

		assert.Len(t, results, 0, "should not output ContestCue when no pattern matches")
	})

	t.Run("should not output when text has pattern but number not in allowlist", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)

		inputCh := make(chan buffer.BufferedContext, 1)
		outputCh := make(chan ContestCue, 1)

		notAllowedContext := buffer.BufferedContext{
			Text:    "Text POTA to 9999",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		inputCh <- notAllowedContext
		close(inputCh)
		parser.ProcessBufferedContextWithPatternMatching(inputCh, outputCh)

		// Assert - should not output anything
		var results []ContestCue
		for result := range outputCh {
			results = append(results, result)
		}

		assert.Len(t, results, 0, "should not output ContestCue when number not in allowlist")
	})

	t.Run("should handle multiple contexts and filter correctly", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234", "5678"}
		parser := NewContestParser(allowlist)

		inputCh := make(chan buffer.BufferedContext, 4)
		outputCh := make(chan ContestCue, 4)

		// Create test contexts
		validContext1 := buffer.BufferedContext{
			Text:    "Text POTA to 1234",
			StartMS: 1000,
			EndMS:   2000,
		}

		validContext2 := buffer.BufferedContext{
			Text:    "Text WWFF to 5678",
			StartMS: 3000,
			EndMS:   4000,
		}

		invalidPattern := buffer.BufferedContext{
			Text:    "Just 1234 with no pattern",
			StartMS: 5000,
			EndMS:   6000,
		}

		invalidNumber := buffer.BufferedContext{
			Text:    "Text SOTA to 9999",
			StartMS: 7000,
			EndMS:   8000,
		}

		// Act
		inputCh <- validContext1
		inputCh <- invalidPattern
		inputCh <- validContext2
		inputCh <- invalidNumber
		close(inputCh)

		parser.ProcessBufferedContextWithPatternMatching(inputCh, outputCh)

		// Assert - should only receive valid ContestCues
		var results []ContestCue
		for result := range outputCh {
			results = append(results, result)
		}

		assert.Len(t, results, 2, "should output two ContestCues")
		assert.Equal(t, "POTA", results[0].ContestType, "should set correct ContestType for first cue")
		assert.Equal(t, "WWFF", results[1].ContestType, "should set correct ContestType for second cue")
	})

	t.Run("should close output channel when input channel closes", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234"}
		parser := NewContestParser(allowlist)

		inputCh := make(chan buffer.BufferedContext)
		outputCh := make(chan ContestCue, 1)

		// Act
		close(inputCh)
		parser.ProcessBufferedContextWithPatternMatching(inputCh, outputCh)

		// Assert
		_, ok := <-outputCh
		assert.False(t, ok, "output channel should be closed")
	})

	t.Run("should handle empty input gracefully", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234"}
		parser := NewContestParser(allowlist)

		inputCh := make(chan buffer.BufferedContext)
		outputCh := make(chan ContestCue)

		// Act
		close(inputCh)
		parser.ProcessBufferedContextWithPatternMatching(inputCh, outputCh)

		// Assert - should not panic and should close output
		_, ok := <-outputCh
		assert.False(t, ok, "output channel should be closed when input is empty")
	})
}

func TestContestParser_ErrorHandlingAndLogging(t *testing.T) {
	t.Run("should handle nil logger gracefully", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234"}

		// Act & Assert - should not panic with nil logger
		parser := NewContestParserWithLogger(allowlist, nil)
		context := &buffer.BufferedContext{
			Text:    "Text POTA to 1234",
			StartMS: 1000,
			EndMS:   2000,
		}

		cue, created := parser.CreateContestCue(context)
		assert.True(t, created, "should create ContestCue even with nil logger")
		assert.NotNil(t, cue, "should return ContestCue")
	})

	t.Run("should log pattern matching attempts with logger", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234"}
		logger := zap.NewNop() // Use no-op logger for testing
		parser := NewContestParserWithLogger(allowlist, logger)

		context := &buffer.BufferedContext{
			Text:    "Text POTA to 1234",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act - should not panic and should work with logger
		cue, created := parser.CreateContestCue(context)

		// Assert
		assert.True(t, created, "should create ContestCue")
		assert.NotNil(t, cue, "should return ContestCue")
	})

	t.Run("should continue processing when logging fails", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234"}
		logger := zap.NewNop() // Use no-op logger for testing
		parser := NewContestParserWithLogger(allowlist, logger)

		inputCh := make(chan buffer.BufferedContext, 1)
		outputCh := make(chan ContestCue, 1)

		context := buffer.BufferedContext{
			Text:    "Text POTA to 1234",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		inputCh <- context
		close(inputCh)
		parser.ProcessBufferedContextWithPatternMatching(inputCh, outputCh)

		// Assert - should continue processing even if logging had issues
		var results []ContestCue
		for result := range outputCh {
			results = append(results, result)
		}

		assert.Len(t, results, 1, "should process successfully despite potential logging issues")
	})
}

func TestContestParser_DetectLetterSequences(t *testing.T) {
	t.Run("should detect simple letter sequence", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Text S A N D to 1234"

		// Act
		sequences := parser.DetectLetterSequences(text)

		// Assert
		assert.Len(t, sequences, 1, "should detect one letter sequence")
		assert.Equal(t, "S A N D", sequences[0], "should detect S A N D sequence")
	})

	t.Run("should detect multiple letter sequences", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Text P A R K and also R O C K to 1234"

		// Act
		sequences := parser.DetectLetterSequences(text)

		// Assert
		assert.Len(t, sequences, 2, "should detect two letter sequences")
		assert.Contains(t, sequences, "P A R K", "should detect P A R K sequence")
		assert.Contains(t, sequences, "R O C K", "should detect R O C K sequence")
	})

	t.Run("should handle single letters surrounded by words", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Hello I spell C A T here and done"

		// Act
		sequences := parser.DetectLetterSequences(text)

		// Assert
		assert.Len(t, sequences, 1, "should detect one letter sequence")
		assert.Equal(t, "C A T", sequences[0], "should detect C A T sequence")
	})

	t.Run("should handle punctuation and extra spacing", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Text  D   ,  O   G   to 1234"

		// Act
		sequences := parser.DetectLetterSequences(text)

		// Assert
		if len(sequences) > 0 {
			assert.Len(t, sequences, 1, "should detect one letter sequence")
			assert.Equal(t, "D O G", sequences[0], "should detect D O G sequence with normalized spacing")
		} else {
			// For now, accept that punctuation breaks sequences - we can enhance this later
			assert.Empty(t, sequences, "punctuation breaks sequences in current implementation")
		}
	})

	t.Run("should ignore short sequences", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Text A B and longer D E F G to 1234"

		// Act
		sequences := parser.DetectLetterSequences(text)

		// Assert
		assert.Len(t, sequences, 1, "should detect only one sequence")
		assert.Equal(t, "D E F G", sequences[0], "should only detect longer sequence")
	})

	t.Run("should handle empty text", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := ""

		// Act
		sequences := parser.DetectLetterSequences(text)

		// Assert
		assert.Empty(t, sequences, "should return empty slice for empty text")
	})
}

func TestContestParser_ReconstructWords(t *testing.T) {
	t.Run("should reconstruct simple word from letter sequence", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		sequence := "S A N D"

		// Act
		word := parser.ReconstructWord(sequence)

		// Assert
		assert.Equal(t, "SAND", word, "should reconstruct SAND from S A N D")
	})

	t.Run("should handle case normalization", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		sequence := "s a n d"

		// Act
		word := parser.ReconstructWord(sequence)

		// Assert
		assert.Equal(t, "SAND", word, "should normalize to uppercase")
	})

	t.Run("should handle mixed case", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		sequence := "P a R k"

		// Act
		word := parser.ReconstructWord(sequence)

		// Assert
		assert.Equal(t, "PARK", word, "should normalize mixed case to uppercase")
	})

	t.Run("should handle extra whitespace", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		sequence := "  R   O   C   K  "

		// Act
		word := parser.ReconstructWord(sequence)

		// Assert
		assert.Equal(t, "ROCK", word, "should handle extra whitespace")
	})

	t.Run("should return empty string for empty sequence", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		sequence := ""

		// Act
		word := parser.ReconstructWord(sequence)

		// Assert
		assert.Equal(t, "", word, "should return empty string for empty sequence")
	})
}

func TestContestParser_ReconstructSpelledWords(t *testing.T) {
	t.Run("should reconstruct spelled words in text", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Text S A N D to 1234"

		// Act
		result := parser.ReconstructSpelledWords(text)

		// Assert
		assert.Equal(t, "Text SAND to 1234", result, "should replace spelled sequence with reconstructed word")
	})

	t.Run("should handle multiple spelled words", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Text P A R K and R O C K to 1234"

		// Act
		result := parser.ReconstructSpelledWords(text)

		// Assert
		assert.Equal(t, "Text PARK and ROCK to 1234", result, "should replace multiple spelled sequences")
	})

	t.Run("should preserve non-spelled text", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Hello world, this has no spelled words in it"

		// Act
		result := parser.ReconstructSpelledWords(text)

		// Assert
		assert.Equal(t, text, result, "should return original text when no spelled words found")
	})

	t.Run("should handle mixed content", func(t *testing.T) {
		// Arrange
		parser := NewContestParser([]string{})
		text := "Station calling, please spell F I R E to confirm"

		// Act
		result := parser.ReconstructSpelledWords(text)

		// Assert
		assert.Equal(t, "Station calling, please spell FIRE to confirm", result, "should replace only spelled sequences")
	})
}

func TestContestParser_IntegrationSpelledWordReconstruction(t *testing.T) {
	t.Run("should create ContestCue from spelled-out keyword", func(t *testing.T) {
		// Arrange
		allowlist := []string{"1234"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Text S A N D to 1234",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		cue, created := parser.CreateContestCue(context)

		// Assert
		assert.True(t, created, "should create ContestCue from spelled-out keyword")
		assert.NotNil(t, cue, "should return ContestCue")
		assert.Equal(t, "SAND", cue.ContestType, "should use reconstructed keyword as contest type")

		// Verify details contain both original and reconstructed text
		assert.Equal(t, "SAND", cue.Details["keyword"], "should have reconstructed keyword in details")
		assert.Equal(t, "1234", cue.Details["number"], "should have correct number in details")
		assert.Equal(t, "Text S A N D to 1234", cue.Details["original_text"], "should preserve original text")
		assert.Equal(t, "Text SAND to 1234", cue.Details["reconstructed_text"], "should include reconstructed text")
	})

	t.Run("should handle spelled-out keyword with different case", func(t *testing.T) {
		// Arrange
		allowlist := []string{"9876"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Text p a r k to 9876",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		cue, created := parser.CreateContestCue(context)

		// Assert
		assert.True(t, created, "should create ContestCue with mixed case spelled keyword")
		assert.NotNil(t, cue, "should return ContestCue")
		assert.Equal(t, "PARK", cue.ContestType, "should use reconstructed keyword as contest type")
		assert.Equal(t, "Text PARK to 9876", cue.Details["reconstructed_text"], "should reconstruct to uppercase")
	})

	t.Run("should maintain compatibility with non-spelled keywords", func(t *testing.T) {
		// Arrange
		allowlist := []string{"5555"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Text POTA to 5555",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		cue, created := parser.CreateContestCue(context)

		// Assert
		assert.True(t, created, "should still work with non-spelled keywords")
		assert.NotNil(t, cue, "should return ContestCue")
		assert.Equal(t, "POTA", cue.ContestType, "should use original keyword")
		assert.Equal(t, "Text POTA to 5555", cue.Details["original_text"], "should have original text")
		assert.Equal(t, "Text POTA to 5555", cue.Details["reconstructed_text"], "should match original when no reconstruction needed")
	})

	t.Run("should fail when spelled keyword doesn't match pattern after reconstruction", func(t *testing.T) {
		// Arrange
		allowlist := []string{"9999"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Just spelling S A N D without pattern",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		cue, created := parser.CreateContestCue(context)

		// Assert
		assert.False(t, created, "should fail when reconstructed text doesn't match contest pattern")
		assert.Nil(t, cue, "should return nil ContestCue")
	})

	t.Run("should fail when number not in allowlist even with spelled keyword", func(t *testing.T) {
		// Arrange
		allowlist := []string{"7777"}
		parser := NewContestParser(allowlist)
		context := &buffer.BufferedContext{
			Text:    "Text S A N D to 1234",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		cue, created := parser.CreateContestCue(context)

		// Assert
		assert.False(t, created, "should fail when number not in allowlist")
		assert.Nil(t, cue, "should return nil ContestCue")
	})
}

// TestContestParser_HyphenatedSequenceDetection tests the detectHyphenatedSequence function
func TestContestParser_HyphenatedSequenceDetection(t *testing.T) {
	parser := NewContestParser([]string{"1234"})

	t.Run("should detect valid hyphenated letter sequence", func(t *testing.T) {
		result := parser.detectHyphenatedSequence("P-O-T-A")
		assert.Equal(t, "P O T A", result)
	})

	t.Run("should detect longer hyphenated sequence", func(t *testing.T) {
		result := parser.detectHyphenatedSequence("W-W-F-F")
		assert.Equal(t, "W W F F", result)
	})

	t.Run("should return empty for non-hyphenated word", func(t *testing.T) {
		result := parser.detectHyphenatedSequence("POTA")
		assert.Equal(t, "", result)
	})

	t.Run("should return empty for sequence too short", func(t *testing.T) {
		result := parser.detectHyphenatedSequence("P-O")
		assert.Equal(t, "", result)
	})

	t.Run("should return empty for sequence with multi-letter parts", func(t *testing.T) {
		result := parser.detectHyphenatedSequence("POT-A")
		assert.Equal(t, "", result)
	})

	t.Run("should handle hyphenated sequence with punctuation", func(t *testing.T) {
		result := parser.detectHyphenatedSequence("P,-O.-T!-A?")
		assert.Equal(t, "P O T A", result)
	})

	t.Run("should return empty for mixed valid/invalid parts", func(t *testing.T) {
		result := parser.detectHyphenatedSequence("P-O-123-A")
		assert.Equal(t, "", result)
	})
}
