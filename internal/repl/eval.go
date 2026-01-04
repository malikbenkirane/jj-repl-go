package reader

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log/slog"
	"strings"
)

type Eval interface {
	Scan(r io.Reader)
}

func NewEval(next chan<- []string, err chan<- error) Eval {
	return &eval{expr: next, err: err}
}

type eval struct {
	expr chan<- []string
	err  chan<- error
}

func (e eval) Scan(r io.Reader) {
	scanner := e.scanner(r)
	var b strings.Builder
	for scanner.Scan() {
		b.WriteString(scanner.Text())
	}
	expr, err := tokens(b.String())
	slog.Info("tokens", "expr", expr, "err", err)
	if err != nil {
		e.err <- err
		return
	}
	e.expr <- expr
}

func (e eval) scanner(r io.Reader) *bufio.Scanner {
	scan := bufio.NewScanner(r)
	scan.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		i := bytes.IndexRune(data, '\n')
		if i == -1 {
			if atEOF {
				return 0, nil, ErrUnexpectedEOF
			}
			return len(data), data, nil
		}
		return 0, data[:i], bufio.ErrFinalToken
	})
	return scan
}

var ErrUnexpectedEOF = errors.New("unexpected EOF")

// tokens naively tokenizes by first extracting double‑quoted segments,
// then splitting the remaining text on spaces.
func tokens(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return []string{}, nil
	}
	parts, err := quoteSplit(s)
	if err != nil {
		return nil, err
	}
	alls := make([][]string, len(parts))
	n := 0
	for i, part := range parts {
		if i%2 == 0 {
			alls[i] = spaceSplit(part)
			n += len(alls[i])
			continue
		}
		n++
	}
	tokens := make([]string, n)
	i := 0
	for j, part := range alls {
		if part == nil {
			tokens[i] = parts[j]
			i++
			continue
		}
		for _, subpart := range part {
			tokens[i] = subpart
			i++
		}
	}
	return tokens, nil
}

// ErrUnbalancedQuotes is returned when a tokenizer encounters mismatched quotation marks.
var ErrUnbalancedQuotes = errors.New("unmatched quotes")
