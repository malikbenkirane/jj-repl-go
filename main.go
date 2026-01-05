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
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	next := make(chan []string)
	err := make(chan error)
	eval := reader.NewEval(next, err)

	var isReading bool

loop:
	for {
		select {
		case <-quit:
			fmt.Println()
			fmt.Println("Bye ❤️")
			return
		case err := <-err:
			fmt.Fprintln(os.Stderr, "<SCAN ERROR>", err)
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
	switch use {
	case ".yac", ".yag", ".y":
		cmd := exec.Command("yac", append([]string{"--no-post", "--debug-prompt"}, expr[1:]...)...)
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		return true, cmd.Run()
	}
	specialHelp()
	return false, fmt.Errorf("unknown command %q", use)
}

func specialHelp() {
	fmt.Println("try .yac or .yag")
}
