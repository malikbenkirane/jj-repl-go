package main

import (
	"fmt"
	"os"
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
			fmt.Println("<ERROR>", err)
		case expr := <-next:
			isReading = false
			fmt.Println("<", expr, ">")
		default:
			if !isReading {
				fmt.Print("> ")
				go eval.Scan(os.Stdin)
				isReading = true
			}
		}
	}
}
