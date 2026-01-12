package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
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
			if err := env.stdAttachCmd("jj", expr...).Run(); err != nil {
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

type sshAgent struct {
	sock string
	key  string
}

type env struct {
	ssh  *sshAgent
	home string
}

// ErrExit is returned when the REPL user requests to exit the session.
var ErrExit = errors.New("EOREPL")

func (env *env) special(expr []string) (found bool, err error) {
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
	case "ssh-agent":
		if env.ssh == nil {
			if err := env.promptSshAgent(os.Stdin); err != nil {
				return true, err
			}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return true, err
		}
		env.home = home
		sshDir := path.Join(home, ".ssh")
		if len(expr) != 2 {
			fmt.Printf("Expected `.ssh-key NAME` (name searched in %s)\n", sshDir)
			return true, err
		}
		name := expr[1]
		if _, err := os.Stat(name); err != nil {
			return true, err
		}
		key := path.Join(sshDir, name)
		cmd := exec.Command("ssh-add", key)
		env.attachAgent(cmd)
		if err := cmd.Run(); err != nil {
			return true, err
		}
		env.ssh.key = key
		return true, nil
	case "exa", "yac":
		cmd := use[1:]
		return true, env.stdAttachCmd(cmd, parseDotArgs(extra[cmd], expr)...).Run()
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
	fmt.Println("")
}

func (env env) attachAgent(cmd *exec.Cmd) {
	if env.ssh != nil {
		cmd.Env = append(cmd.Env, "HOME="+env.home)
		cmd.Env = append(cmd.Env, "SSH_AUTH_SOCK="+env.ssh.sock)
		cmd.Env = append(cmd.Env, "SSH_ENV="+path.Join(env.home, ".ssh", "environment"))
	}
}

func (env env) stdAttachCmd(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	env.attachAgent(cmd)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	fmt.Println(cmd.Env)
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

func (env *env) promptSshAgent(from io.Reader) error {
	r := bufio.NewReader(from)
	fmt.Print("Agent Sock: ")
	sock, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if env.ssh == nil {
		env.ssh = new(sshAgent)
	}
	env.ssh.sock = sock
	return nil
}
