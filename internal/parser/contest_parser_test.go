package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"

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