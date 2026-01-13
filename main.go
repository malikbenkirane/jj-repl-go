package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	reader "github.com/4sp1/jrl/internal/repl"
)

func main() {
	cancel := make(chan os.Signal, 1)
	signal.Notify(cancel, syscall.SIGTERM, syscall.SIGINT)

	next := make(chan []string)
	defer close(next)

	err := make(chan error)
	defer close(err)

	eval := reader.NewEval(next, err)

	var isReading bool

	var env env
	env.history = &history{
		entries: make([][]string, 4),
		size:    4,
	}

	env.stash = &stash{}

loop:
	for {
		select {
		case <-cancel:
			isReading = false
			fmt.Println()
		case err := <-err:
			isReading = false
			fmt.Fprintln(os.Stderr, "<SCAN ERROR>", err)
		case expr := <-next:
			isReading = false
			found, err := env.special(expr)
			if err != nil {
				if errors.Is(err, ErrExit) {
					fmt.Println()
					fmt.Println("Bye ❤️")
					return
				}
				fmt.Fprintln(os.Stderr, "<SPECIAL CMD ERROR>", err)
				continue loop
			}
			if found {
				fmt.Println("<SPECIAL CMD 👍>")
				continue loop
			}
			cmd := exec.Command("jj", expr...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "<CMD ERROR>", err)
			}
			if len(expr) > 1 {
				env.save(expr)
			}
		default:
			if !isReading {
				fmt.Print("> ")
				go eval.Scan(os.Stdin)
				isReading = true
			}
		}
	}
}

type env struct {
	history *history
	stash   *stash
}

type stash struct {
	path string
}

func (s stash) isOpen() bool {
	return len(s.path) > 0
}

func (s stash) isNotOpen() bool {
	return len(s.path) == 0
}

type history struct {
	index   int
	size    int
	entries [][]string
}

func (env env) save(expr []string) {
	h := env.history
	if h.index == h.size-1 {
		for i := range h.size - 1 {
			h.entries[i] = h.entries[i+1]
		}
	} else {
		h.index++
	}
	h.entries[h.index] = expr
}

// ErrExit is returned when the REPL user requests to exit the session.
var ErrExit = errors.New("EOREPL")

func (env env) special(expr []string) (found bool, err error) {
	if len(expr) == 0 {
		return false, nil
	}
	use := expr[0]
	if len(use) == 0 {
		return false, nil
	}
	if use[0] == '.' && len(use) == 1 {
		return true, nil
	}
	if use[0] != '.' {
		return false, nil
	}
	extra := map[string]map[string][]string{
		"yac": {
			"prompt": {"--no-post", "--debug-prompt"},
		},
		"exa": {
			"tree": {"-T"},
		},
	}
	switch use[1:] {
	case "quit", "exit", "bye":
		return true, ErrExit
	case "stash":
		if len(expr) == 2 {
			switch expr[1] {
			case ".write":
				if env.stash.isNotOpen() {
					fmt.Println("⚠️ No stash is currently open. Use `.stash [FILE]` to start one.")
					return true, nil
				}
				f, err := os.Open(env.stash.path)
				if err != nil {
					return true, err
				}
				defer func() {
					if err := f.Close(); err != nil {
						fmt.Printf("⚠️ unable to close stash: %v", err)
					}
				}()
				cmd := exec.Command("jj", "describe", "--stdin")
				cmd.Stdin = f
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return true, err
				}
				fmt.Println("Great! Your stash is saved. Use `.stash .drop` to clear it when you're ready.")
				return true, nil
			case ".drop":
				if env.stash.isNotOpen() {
					fmt.Println("⚠️ No stash is currently open. Use `.stash [FILE]` to start one.")
				}
				fmt.Println("Great! Your stash is dropped. You will need to manually remove it.")
				fmt.Println("Your stash is located at", env.stash.path)
				env.stash.path = ""
				return true, nil
			default:
				if env.stash.isOpen() {
					fmt.Println("⚠️ Open stash in use. Please run `.stash .drop` first.")
					return true, nil
				}
				f, err := os.OpenFile(expr[1], os.O_APPEND|os.O_CREATE, 0600)
				if err != nil {
					return true, err
				}
				defer func() {
					if err := f.Close(); err != nil {
						fmt.Printf("⚠️ unable to close stash: %v", err)
					}
				}()
				env.stash.path = expr[1]
				fmt.Println("Good, your commit stash now points to", expr[1])
				return true, nil
			}
		} else if len(expr) != 1 {
			fmt.Println("Usage: `.stash [FILE]` to start one, `.stash .write` to commit or `.stash .drop`.")
			return true, nil
		}
		if env.stash.isOpen() {
			fmt.Println("⚠️ Open stash in use. Please run `.stash .drop` first.")
			return true, nil
		}
		f, err := os.CreateTemp("", "commit-stash-*")
		if err != nil {
			return true, err
		}
		defer func() {
			if err := f.Close(); err != nil {
				fmt.Printf("⚠️ unable to close stash: %v", err)
			}
		}()
		fmt.Println("Good, your commit stash now points to", f.Name())
		env.stash.path = f.Name()
		return true, nil

	case "history":
		for i := range env.history.index + 1 {
			atoms := make([]string, len(env.history.entries[i]))
			for i, atom := range env.history.entries[i] {
				atoms[i] = fmt.Sprintf(`"%s"`, atom)
			}
			fmt.Println(strings.Join(atoms, " "))
		}
		return true, nil
	case "sh":
		shell := "sh"
		{
			userShell, found := os.LookupEnv("SHELL")
			if found {
				shell = userShell
			}
		}
		return true, stdAttachCmd(shell).Run()
	case "exa", "yac":
		cmd := use[1:]
		return true, stdAttachCmd(cmd, parseDotArgs(extra[cmd], expr)...).Run()
	case "ignore":
		f, err := os.OpenFile(".gitignore", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return true, err
		}
		defer func() {
			if err := f.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "⚠️ unable to close .gitignore")
			}
		}()
		for _, name := range expr[1:] {
			if _, err := os.Stat(name); err != nil {
				fmt.Fprintln(os.Stderr, err, "unable to proceed with", name)
				continue
			}
			fmt.Println("adding", name, "to .gitignore")
			if _, err := fmt.Fprintln(f, name); err != nil {
				fmt.Fprintln(os.Stderr, err, "unable to proceed with", name)
			}
			fmt.Println("✅", name, "added to .gitignore")
		}
		return true, nil
	default:
		if len(use) >= 2 && use[1] == '.' {
			n := 4
			if len(use) >= 3 {
				n, err = strconv.Atoi(use[2:])
				if err != nil {
					return true, err
				}
			}
			return true, stdAttachCmd("jj", "log", "--limit", strconv.Itoa(n)).Run()
		}
	}
	specialHelp()
	return false, fmt.Errorf("unknown command %q", use)
}

func specialHelp() {
	fmt.Println("")
}

func stdAttachCmd(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd
}

func parseDotArgs(dotArgs map[string][]string, expr []string) (args []string) {
	if len(expr) >= 2 && len(expr[1]) > 0 && expr[1][0] == '.' {
		prepend, ok := dotArgs[expr[1][1:]]
		if ok {
			return append(prepend, expr[2:]...)
		}
	}
	return expr[1:]
}
