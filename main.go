package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	reader "github.com/4sp1/jrl/internal/repl"
)

func main() {
	cancel := make(chan os.Signal, 1)
	signal.Notify(cancel, syscall.SIGTERM, syscall.SIGINT)

	next := make(chan []string)
	err := make(chan error)
	eval := reader.NewEval(next, err)

	var isReading bool

loop:
	for {
		select {
		case <-cancel:
			fmt.Println()
		case err := <-err:
			fmt.Fprintln(os.Stderr, "<SCAN ERROR>", err)
			isReading = true
		case expr := <-next:
			isReading = false
			found, err := special(expr)
			if err != nil {
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
		default:
			if !isReading {
				fmt.Print("> ")
				go eval.Scan(os.Stdin)
				isReading = true
			}
		}
	}
}

func special(expr []string) (found bool, err error) {
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
	}
	specialHelp()
	return false, fmt.Errorf("unknown command %q", use)
}

func specialHelp() {
	fmt.Println("try .yac or .yag")
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
