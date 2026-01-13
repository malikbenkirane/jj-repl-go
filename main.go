package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
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

	var env *env
	{
		newEnv, err := initEnv()
		if err != nil {
			fmt.Println("initEnv:", err)
			os.Exit(1)
		}
		env = newEnv
	}

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
	cache   string
}

func initEnv() (*env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	env := &env{
		history: &history{
			size:    4,
			entries: make([][]string, 4),
		},
		cache: path.Join(home, ".cache", "jrl"),
	}
	if err := os.MkdirAll(env.cache, 0700); err != nil {
		return nil, err
	}
	stash, err := env.initStash()
	if err != nil {
		return nil, fmt.Errorf("initStash: %w", err)
	}
	env.stash = stash
	return env, nil
}

func (env env) initStash() (*stash, error) {
	stash := &stash{}
	stash.cacheFile = path.Join(env.cache, "stash")
	_, err := os.Stat(stash.cacheFile)
	if os.IsNotExist(err) {
		stash.path = ""
		return stash, nil
	}
	if err != nil {
		return nil, err
	}
	if err := stash.cacheRead(); err != nil {
		return nil, err
	}
	return stash, nil
}

type stash struct {
	path      string
	cacheFile string
}

var ErrStashIsNotOpen = errors.New("stash is not open")

func (stash *stash) drop() error {
	_, err := os.Stat(stash.cacheFile)
	if os.IsNotExist(err) {
		return ErrStashIsNotOpen
	}
	if err != nil {
		return err
	}
	if err := os.Remove(stash.cacheFile); err != nil {
		return err
	}
	stash.path = ""
	return nil
}

type ErrStashIsOpen struct {
	file string
}

func (err *ErrStashIsOpen) Error() string {
	return ("stash is open")
}

func (stash *stash) cacheRead() error {
	f, err := os.Open(stash.cacheFile)
	if err != nil {
		return err
	}
	defer func() {
		err = f.Close()
	}()
	var b bytes.Buffer
	if _, err := io.Copy(&b, f); err != nil {
		return err
	}
	stash.path = strings.TrimSpace(b.String())
	if stash.path == "" {
		return fmt.Errorf("empty path")
	}
	return nil
}

func (stash *stash) cachePath(file string) error {
	_, err := os.Stat(stash.cacheFile)
	if os.IsNotExist(err) {
		f, err := os.Create(stash.cacheFile)
		if err != nil {
			return err
		}
		if _, err := f.WriteString(file); err != nil {
			return err
		}
		stash.path = file
		return nil
	}
	if err != nil {
		return err
	}
	if err := stash.cacheRead(); err != nil {
		return err
	}
	return &ErrStashIsOpen{stash.path}
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
			"stash":  {"--debug-prompt"},
		},
		"exa": {
			"tree": {"-T"},
		},
	}
	switch use[1:] {
	case "quit", "exit", "bye":
		return true, ErrExit
	case "stash":
		if len(expr) >= 2 {
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
				args := []string{"describe", "--stdin"}
				if len(expr) >= 3 {
					args = append(args, expr[2:]...)
				}
				cmd := exec.Command("jj", args...)
				cmd.Stdin = f
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return true, err
				}
				fmt.Println("Great! Your stash is saved. Use `.stash .drop` to clear it when you're ready.")
				return true, nil
			case ".drop":
				file := env.stash.path
				err := env.stash.drop()
				if errors.Is(err, ErrStashIsNotOpen) {
					fmt.Println("⚠️ No stash is currently open. Use `.stash [FILE]` to start one.")
					return true, nil
				}
				fmt.Println("Great! Your stash is dropped. You will need to manually remove it.")
				fmt.Println("Your stash is located at", file)
				return true, nil
			default:
				err := env.stash.cachePath(expr[1])
				var errOpen *ErrStashIsOpen
				if errors.As(err, &errOpen) {
					fmt.Println("⚠️ Open stash in use. Please run `.stash .drop` first.")
					fmt.Println("Stash currently points to", errOpen.file)
					return true, nil
				}
				if err != nil {
					return true, err
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
		f, err := os.CreateTemp("", "commit-stash-*")
		if err != nil {
			return true, err
		}
		defer func() {
			if err := f.Close(); err != nil {
				fmt.Printf("⚠️ unable to close stash: %v", err)
			}
		}()
		{
			err := env.stash.cachePath(f.Name())
			var errOpen *ErrStashIsOpen
			if errors.As(err, &errOpen) {
				fmt.Println("⚠️ Open stash in use. Please run `.stash .drop` first.")
				fmt.Println("Stash currently points to", errOpen.file)
				if err := os.Remove(f.Name()); err != nil {
					return true, err
				}
				return true, nil
			}
		}
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
		if len(expr) > 1 {
			atoms := make([]string, len(expr[1:]))
			for i, atom := range expr[1:] {
				atoms[i] = fmt.Sprintf(`"%s"`, atom)
			}
			return true, stdAttachCmd(shell, "-c", strings.Join(atoms, " ")).Run()
		}
		return true, stdAttachCmd(shell).Run()

	case "exa", "yac":
		cmd := use[1:]
		args := parseDotArgs(extra[cmd], expr)
		if cmd == "yac" && args.cmd == "stash" {
			// The current dot‑args implementation lacks generic support for this case.
			// Manually construct the arguments needed for this command, which requires
			// a specific formatting and custom handling.
			if env.stash.isNotOpen() {
				fmt.Println("⚠️ No stash is currently open. Use `.stash [FILE]` to start one.")
				return true, nil
			}
			// yac's `--stdout` flag is currently broken (see https://github.com/4sp1/yac/issues/3)
			// so we capture the command's output manually.
			// The intended logic would be:
			//
			// args.args = append([]string{"--stdout", env.stash.path}, args.args...)
			//
			// but for now we write directly to the stash file.
			cmd := exec.Command("yac", "--debug-prompt")
			if err := func() (err error) {
				f, err := os.OpenFile(env.stash.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
				if err != nil {
					return err
				}
				defer func() {
					err = f.Close()
				}()
				cmd.Stdout = f
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return err
				}
				fmt.Println("Written to", f.Name())
				return nil
			}(); err != nil {
				return true, err
			}
			return true, nil
		}
		return true, stdAttachCmd(cmd, args.args...).Run()

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

type dotArgs struct {
	args []string
	cmd  string
}

func parseDotArgs(dotArgs map[string][]string, expr []string) (args dotArgs) {
	if len(expr) >= 2 && len(expr[1]) > 0 && expr[1][0] == '.' {
		cmd := expr[1][1:]
		prepend, ok := dotArgs[cmd]
		if ok {
			args.cmd = cmd
			args.args = append(prepend, expr[2:]...)
			return args
		}
	}
	args.args = expr[1:]
	return args
}
