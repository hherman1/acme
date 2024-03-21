package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"strings"
)

var concurrency = flag.Uint("c", 100, "how many lines to process concurrently (default: 100)")
var argname = flag.String("a", "ARG", "name of the argument to replace in the command")

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()
	flag.Parse()
	chansize := *concurrency
	semaphore := make(chan struct{}, chansize)
	type r struct {
		Out []byte
		Err error
	}
	results := make(chan r)
	scanner := bufio.NewScanner(os.Stdin)
	if len(flag.Args()) == 0 {
		return fmt.Errorf("no command given")
	}
	cmdname := flag.Args()[0]
	go func() {
		done := make(chan struct{})
		count := 0
		for scanner.Scan() {
			count++
			semaphore <- struct{}{}
			go func(line string) {
				defer func() { <-semaphore; done <- struct{}{} }()
				args := slices.Clone(flag.Args()[1:])
				for i, a := range args {
					args[i] = strings.ReplaceAll(a, *argname, strings.TrimSpace(line))
				}
				out, err := exec.CommandContext(ctx, cmdname, args...).CombinedOutput()
				results <- r{Out: out, Err: err}
			}(scanner.Text())
		}
		for i := 0; i < count; i++ {
			<-done
		}
		close(results)
	}()

	for rr := range results {
		if rr.Err != nil {
			return rr.Err
		}
		fmt.Print(string(rr.Out))
	}
	return nil
}
