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

	for {
		select {
		case <-quit:
			fmt.Println()
			fmt.Println("Bye ❤️")
			return
		case err := <-err:
			fmt.Fprintln(os.Stderr, "<SCAN ERROR>", err)
			isReading = true
		case expr := <-next:
			cmd := exec.Command("jj", expr...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "<CMD ERROR>", err)
			}
			isReading = false
		default:
			if !isReading {
				fmt.Print("> ")
				go eval.Scan(os.Stdin)
				isReading = true
			}
		}
	}
}
