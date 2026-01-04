package reader

import (
	"errors"
	"strings"
	"testing"
)

func TestTokens(t *testing.T) {
	for _, c := range []struct {
		name     string
		input    string
		expected []string
		err      error
	}{
		{
			name:     "empty",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only spaces",
			input:    "     ",
			expected: []string{},
		},
		{
			name:     "single",
			input:    "single",
			expected: []string{"single"},
		},
		{
			name:     "no quotes",
			input:    "  p    a sample  with various    spaces",
			expected: []string{"p", "a", "sample", "with", "various", "spaces"},
		},
		{
			name:  "unbalanced quotes single",
			input: `"unbalanced quotes`,
			err:   ErrUnbalancedQuotes,
		},
		{
			name:  "unbalanced quotes multiples",
			input: `"unbalanced quotes"    "mul " tit"`,
			err:   ErrUnbalancedQuotes,
		},
		{
			name:  "mixed",
			input: `  this is "a mixed"  sample "various spaces"  and multiple "double quotes"`,
			expected: []string{
				"this", "is", "a mixed", "sample",
				"various spaces", "and", "multiple", "double quotes",
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := tokens(c.input)
			if c.err != nil && !errors.Is(err, c.err) {
				t.Fatalf("expected err %[1]T(%[1]q) got %[2]T(%[2]q)", c.err, err)
			}
			if c.err != nil {
				return
			}
			if err != nil {
				t.Fatalf("unexpected err %q", err)
			}
			if len(got) != len(c.expected) {
				t.Fatalf("expected %d token got %d", len(c.expected), len(got))
			}
			failed := false
			for i := range len(got) {
				if got[i] != c.expected[i] {
					failed = true
					t.Logf("expected %[1]s at %[3]d got %[2]s", c.expected[i], got[i], i)
				}
			}
			if failed {
				t.Fatalf("expected %v got %v", c.expected, got)
			}
		})
	}
}

func TestQuoteSplit(t *testing.T) {
	for _, c := range []struct {
		name     string
		input    string
		expected []string
		err      error
	}{
		{
			name:     "start with quote",
			input:    `"start" balanced`,
			expected: []string{"start", " balanced"},
		},
		{
			name:     "end with quote",
			input:    `"start" "balanced"`,
			expected: []string{"start", "balanced"},
		},
		{
			name:     "no quote",
			input:    `no quote`,
			expected: []string{"no quote"},
		},
		{
			name:  "unbalanced 1",
			input: `"unbalanced`,
			err:   ErrUnbalancedQuotes,
		},
		{
			name:  "unbalanced 2",
			input: `this "unbalanced" two "some left`,
			err:   ErrUnbalancedQuotes,
		},
		{
			name:     "balanced",
			input:    `this is "balanced" "with multiple" quote and "unquoted"`,
			expected: []string{"this is ", "balanced", "with multiple", " quote and ", "unquoted"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := quoteSplit(c.input)
			if c.err != nil && !errors.Is(err, c.err) {
				t.Fatalf("expected err %[1]T(%[1]q) got %[2]T(%[2]q)", c.err, err)
			}
			if c.err != nil {
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %q", err)
			}
			if len(got) != len(c.expected) {
				t.Fatalf("expected %d token got %d", len(c.expected), len(got))
			}
			failed := false
			for i := range len(got) {
				if got[i] != c.expected[i] {
					failed = true
					t.Logf("expected %s at %d got %s", c.expected[i], i, got[i])
				}
				if failed {
					t.Fatalf("expected %v got %v", c.expected, got)
				}
			}
		})
	}
}

func TestDoubleQuoteSplit(t *testing.T) {
	for _, c := range []struct {
		name     string
		input    string
		expected []string
	}{
		{
			"start with quote",
			`"hi" start with "quote"`,
			[]string{"", "hi", " start with ", "quote", ""},
		},
		{
			"single quote",
			`"single`,
			[]string{"", "single"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Split(c.input, `"`)
			if len(c.expected) != len(got) {
				t.Fatalf("expected %d parts got %d", len(c.expected), len(got))
			}
			for i, part := range got {
				if part != c.expected[i] {
					t.Fatalf("expected %v got %v", c.expected, got)
				}
			}
		})
	}
}

func TestSpaceSplit(t *testing.T) {
	for _, c := range []struct {
		name     string
		input    string
		expected []string
	}{
		{
			"repeated spaces",
			"hi   some more   space",
			[]string{"hi", "some", "more", "space"},
		},
		{
			"start or end with spaces",
			"    hi some space at    start and end  ",
			[]string{"hi", "some", "space", "at", "start", "and", "end"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := spaceSplit(c.input)
			if len(c.expected) != len(got) {
				t.Fatalf("expected %d parts got %d", len(c.expected), len(got))
			}
			for i, part := range got {
				if part != c.expected[i] {
					t.Fatalf("expected %v got %v", c.expected, got)
				}
			}
		})
	}
}
