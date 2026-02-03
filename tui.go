package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Mode int

const (
	ModeInsert Mode = iota
	ModeNormal
)

type Model struct {
	input           string
	cursor          int
	isCursorVisible bool
	mode            Mode
	debug           string
}

type TickMsg time.Time

func cursorTick(isCursorVisible bool) tea.Cmd {
	d := 250 * time.Millisecond
	if !isCursorVisible {
		d = 800 * time.Millisecond
	}
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return cursorTick(m.isCursorVisible)
}

func nextWord(cursor int, input string) int {
	if cursor >= 0 && cursor < len(input) {
		sub := input[cursor:]
		s := strings.IndexByte(sub, ' ')
		if s >= 0 {
			sub := input[cursor+s:]
			p := strings.IndexFunc(sub, func(r rune) bool {
				return r != ' '
			})
			debug = fmt.Sprintln("nextWord:", "p =", p, "sub =", "<<", sub, ">>")
			if p > 0 {
				return p + s
			}
		}
	}
	return 0
}

var debug string

func endWord(cursor int, input string) int {
	debug = fmt.Sprintln("endWord", "cursor =", cursor, "input = <<", input, ">>")
	if cursor >= 0 && cursor < len(input) {
		sub := input[cursor:]
		debug += fmt.Sprintln("sub = <<", sub, ">>")
		s := strings.IndexByte(sub, ' ')
		debug += fmt.Sprintln("s =", s)
		switch {
		case s > 1:
			return s - 1
		case s == -1:
			return len(sub) - 1
		default:
			deltaNext := nextWord(cursor, input)
			deltaEnd := endWord(cursor+deltaNext, input)
			return deltaNext + deltaEnd
		}
	}
	return 0
}

func reverseInput(input string) string {
	runes := []rune(input)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.input = "there is more to see"
	m.mode = ModeNormal
	switch msg := msg.(type) {
	case TickMsg:
		m.isCursorVisible = !m.isCursorVisible
		return m, cursorTick(m.isCursorVisible)
	case tea.KeyMsg:
		if m.mode == ModeInsert && len(msg.String()) == 1 {
			var b strings.Builder
			for i := range len(m.input) + 1 {
				if i == m.cursor {
					b.WriteString(msg.String())
				}
				if i < len(m.input) {
					b.WriteRune(rune(m.input[i]))
				}
			}
			m.input = b.String()
			m.cursor++
		}
		if m.mode == ModeNormal {
			switch msg.String() {
			case "w":
				m.cursor += nextWord(m.cursor, m.input)
			case "e":
				m.cursor += endWord(m.cursor, m.input)
			case "b":
				m.cursor -= endWord(len(m.input)-1-m.cursor, reverseInput(m.input))
			case "a":
				m.cursor++
				m.mode = ModeInsert
			case "i":
				m.mode = ModeInsert
			case "I":
				m.cursor = 0
				m.mode = ModeInsert
			case "A":
				m.cursor = len(m.input)
				m.mode = ModeInsert
			case "d":
				m = m.deleteChar()
				m.cursor--
			case "r":
				m = m.deleteChar()
				m.mode = ModeInsert
			case "h":
				m.cursor--
			case "l":
				m.cursor++
			case "0":
				m.cursor = 0
			case "$":
				m.cursor = len(m.input)
			}
		}
		switch msg.String() {
		case tea.KeyEsc.String():
			m.mode = ModeNormal
		case tea.KeyBackspace.String():
			if m.mode == ModeInsert {
				m = m.deleteChar()
			}
			m.cursor--
		case tea.KeyLeft.String():
			m.cursor--
		case tea.KeyRight.String():
			m.cursor++
		}
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > len(m.input) {
		m.cursor = len(m.input)
	}
	m.isCursorVisible = true
	return m, nil
}

func (m Model) deleteChar() Model {
	at := m.cursor - 1
	if m.mode == ModeNormal {
		at = m.cursor
	}
	var b strings.Builder
	for i, r := range m.input {
		if i != at {
			b.WriteRune(r)
		}
	}
	m.input = b.String()
	return m
}

const (
	cursorInsert = '🭻'
	cursorBlock  = '█'
)

func (m Model) View() string {
	var s strings.Builder
	s.WriteString("> ")
	for i := range len(m.input) + 1 {
		if i == m.cursor {
			if m.isCursorVisible {
				switch m.mode {
				case ModeInsert:
					s.WriteRune(cursorInsert)
				case ModeNormal:
					s.WriteRune(cursorBlock)
				}
			} else if i < len(m.input) {
				s.WriteByte(m.input[i])
			}
		} else {
			if i < len(m.input) {
				s.WriteByte(m.input[i])
			}
		}
	}
	m.debug = debug
	s.WriteByte('\n')
	s.WriteString(m.debug)
	return s.String()
}
