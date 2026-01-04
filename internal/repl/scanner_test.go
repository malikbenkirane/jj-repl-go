package reader

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewEval(t *testing.T) {
	next, err := make(chan []string, 1), make(chan error, 1)
	e := NewEval(next, err)
	eval, ok := e.(*eval)
	if !ok {
		t.Fatalf("unexpected type %T exepected *eval", e)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-ctx.Done():
			t.Fatalf("test ended before asserts")
		default:
			eval.err <- ErrUnbalancedQuotes
			eval.expr <- []string{"expr"}
		}
	})
	wg.Go(func() {
		select {
		case <-ctx.Done():
			t.Fatalf("test ended before asserts")
		case err := <-err:
			if err != ErrUnbalancedQuotes {
				t.Fatalf("unexpected eval.err assignment")
			}
		}
	})
	wg.Go(func() {
		select {
		case <-ctx.Done():
			t.Fatalf("test ended before asserts")
		case expr := <-next:
			if len(expr) != 1 {
				t.Fatalf("unexpected eval.expr assignment")
			}
			if expr[0] != "expr" {
				t.Fatalf("unexpected eval.expr assignment")
			}
		}
	})
	wg.Wait()
}

func Test_eval_Scan(t *testing.T) {
	for _, c := range []struct {
		name     string
		input    string
		expected []string
		err      error
	}{
		{
			name:  "unbalanced quotes",
			input: "\"unbalanced",
			err:   ErrUnbalancedQuotes,
		},
		{
			name:     "should pass",
			input:    "sample see other \"tests for more\" examples  \"👋\"",
			expected: []string{"sample", "see", "other", "tests for more", "examples", "👋"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			next, err := make(chan []string, 1), make(chan error, 1)
			eval := NewEval(next, err)
			eval.Scan(strings.NewReader(c.input))
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			var wg sync.WaitGroup
			wg.Go(func() {
				select {
				case <-ctx.Done():
					if c.err != nil {
						t.Fatalf("unexpected end of test, expected error %q", c.err)
					}
				case err := <-err:
					if c.err == nil {
						t.Fatalf("unexpected error %q", err)
					}
					if !errors.Is(err, c.err) {
						t.Fatalf("expected error %[1]T(%[1]q) got %[2]T(%[2]q)", c.err, err)
					}
				}
			})
			wg.Go(func() {
				select {
				case <-ctx.Done():
					if c.err == nil {
						t.Fatalf("unexpected end of test, expected expr %v", c.expected)
					}
				case expr := <-next:
					if len(expr) != len(c.expected) {
						t.Fatalf("expected %v got %v", c.expected, expr)
					}
					for i := range c.expected {
						if c.expected[i] != expr[i] {
							t.Logf("expected %q at %d got %q", c.expected[i], i, expr[i])
							t.Fail()
						}
					}
				}
			})
			wg.Wait()
		})
	}
}

func Test_eval_scanner(t *testing.T) {
	for _, c := range []struct {
		name     string
		input    string
		expected string
		err      error
	}{
		{
			name:     "terminates",
			input:    "this terminates \n",
			expected: "this terminates ",
		},
		{
			name:  "unexpected EOF",
			input: "this does not terminate",
			err:   ErrUnexpectedEOF,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			scanner := eval{}.scanner(strings.NewReader(c.input))
			var b strings.Builder
			for scanner.Scan() {
				b.WriteString(scanner.Text())
			}
			err := scanner.Err()
			if c.err != nil && !errors.Is(err, c.err) {
				t.Fatalf("expected err %[1]T(%[1]q) got %[2]T(%[2]q)", c.err, err)
			}
			if c.err != nil {
				return
			}
			if err != nil {
				t.Fatalf("unexpected err %q", err)
			}
			if b.String() != c.expected {
				t.Fatalf("expected %q got %q", c.expected, b.String())
			}
		})
	}
}
