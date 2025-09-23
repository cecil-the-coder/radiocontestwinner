package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"radiocontestwinner/internal/parser"
)

// Tests for spelled-out keyword reconstruction functionality

func TestSpelledWords_DetectLetterSequences(t *testing.T) {
	contestParser := parser.NewContestParser([]string{"12345"})

	testCases := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "basic spelled word",
			text:     "C O N T E S T",
			expected: []string{"C O N T E S T"},
		},
		{
			name:     "spelled word with punctuation",
			text:     "C-O-N-T-E-S-T",
			expected: nil, // Hyphenated words are parsed as single tokens, not letter sequences
		},
		{
			name:     "spelled word in sentence",
			text:     "Text C O N T E S T to 12345",
			expected: []string{"C O N T E S T"},
		},
		{
			name:     "multiple spelled words",
			text:     "R A D I O and C O N T E S T",
			expected: []string{"R A D I O", "C O N T E S T"},
		},
		{
			name:     "short sequence (ignored)",
			text:     "A B and some text",
			expected: nil, // Too short (< 3 letters)
		},
		{
			name:     "mixed with regular words",
			text:     "The word C O N T E S T is spelled out",
			expected: []string{"C O N T E S T"},
		},
		{
			name:     "no letter sequences",
			text:     "This is regular text with no spelling",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sequences := contestParser.DetectLetterSequences(tc.text)
			assert.Equal(t, tc.expected, sequences)
		})
	}
}

func TestSpelledWords_ReconstructWord(t *testing.T) {
	contestParser := parser.NewContestParser([]string{"12345"})

	testCases := []struct {
		name     string
		sequence string
		expected string
	}{
		{
			name:     "basic sequence",
			sequence: "C O N T E S T",
			expected: "CONTEST",
		},
		{
			name:     "sequence with punctuation",
			sequence: "C-O-N-T-E-S-T",
			expected: "", // This input is treated as a single token, not a sequence
		},
		{
			name:     "lowercase sequence",
			sequence: "c o n t e s t",
			expected: "CONTEST",
		},
		{
			name:     "mixed case sequence",
			sequence: "c O n T e S t",
			expected: "CONTEST",
		},
		{
			name:     "empty sequence",
			sequence: "",
			expected: "",
		},
		{
			name:     "single letter",
			sequence: "C",
			expected: "C",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := contestParser.ReconstructWord(tc.sequence)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSpelledWords_ReconstructSpelledWords(t *testing.T) {
	contestParser := parser.NewContestParser([]string{"12345"})

	testCases := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "single spelled word",
			text:     "Text C O N T E S T to 12345",
			expected: "Text CONTEST to 12345",
		},
		{
			name:     "spelled word with punctuation",
			text:     "Text C-O-N-T-E-S-T to 12345",
			expected: "Text CONTEST to 12345",
		},
		{
			name:     "multiple spelled words",
			text:     "R A D I O contest C O N T E S T to 12345",
			expected: "RADIO contest CONTEST to 12345",
		},
		{
			name:     "no spelled words",
			text:     "Text CONTEST to 12345",
			expected: "Text CONTEST to 12345", // No change
		},
		{
			name:     "short sequences ignored",
			text:     "A B is too short but C O N T E S T works",
			expected: "A B is too short but CONTEST works",
		},
		{
			name:     "empty text",
			text:     "",
			expected: "",
		},
		{
			name:     "complex sentence",
			text:     "Please text the word C-O-N-T-E-S-T to the number 12345 for R A D I O prizes",
			expected: "Please text the word CONTEST to the number 12345 for RADIO prizes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := contestParser.ReconstructSpelledWords(tc.text)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSpelledWords_EndToEndIntegration(t *testing.T) {
	t.Run("should successfully process spelled-out contest patterns", func(t *testing.T) {
		allowlist := []string{"12345", "67890"}
		contestParser := parser.NewContestParserWithLogger(allowlist, nil)

		testCases := []struct {
			name            string
			originalText    string
			expectMatch     bool
			expectedKeyword string
			expectedNumber  string
		}{
			{
				name:            "spelled contest keyword",
				originalText:    "Text C-O-N-T-E-S-T to 12345",
				expectMatch:     true,
				expectedKeyword: "CONTEST",
				expectedNumber:  "12345",
			},
			{
				name:            "spelled radio keyword",
				originalText:    "Text R A D I O to 67890",
				expectMatch:     true,
				expectedKeyword: "RADIO",
				expectedNumber:  "67890",
			},
			{
				name:            "spelled keyword with non-allowlisted number",
				originalText:    "Text C-O-N-T-E-S-T to 99999",
				expectMatch:     false,
				expectedKeyword: "",
				expectedNumber:  "",
			},
			{
				name:            "mixed spelled and regular words",
				originalText:    "Text C O N T E S T to 12345 on the radio",
				expectMatch:     true,
				expectedKeyword: "CONTEST",
				expectedNumber:  "12345",
			},
			{
				name:            "no pattern match after reconstruction",
				originalText:    "Just some C O N T E S T words without pattern",
				expectMatch:     false,
				expectedKeyword: "",
				expectedNumber:  "",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Test pattern matching with spelled word reconstruction
				keyword, number, matched := contestParser.MatchContestPattern(tc.originalText)

				if tc.expectMatch {
					assert.True(t, matched, "Expected pattern to match for: %s", tc.originalText)
					assert.Equal(t, tc.expectedKeyword, keyword)
					assert.Equal(t, tc.expectedNumber, number)
				} else {
					assert.False(t, matched, "Expected pattern to NOT match for: %s", tc.originalText)
				}

				// Also test reconstruction directly
				reconstructed := contestParser.ReconstructSpelledWords(tc.originalText)
				t.Logf("Original: %s", tc.originalText)
				t.Logf("Reconstructed: %s", reconstructed)

				// Verify reconstruction improves pattern matching where appropriate
				if tc.expectMatch {
					assert.Contains(t, reconstructed, tc.expectedKeyword, "Reconstructed text should contain expected keyword")
				}
			})
		}
	})
}
