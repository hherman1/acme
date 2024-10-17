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
var sep = flag.String("F", ",", "field separator (COL[N]), default is comma")

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
	results := make(chan []byte)
	scanner := bufio.NewScanner(os.Stdin)
	if len(flag.Args()) == 0 {
		return fmt.Errorf("no command given")
	}
	go func() {
		done := make(chan struct{})
		count := 0
		for scanner.Scan() {
			count++
			semaphore <- struct{}{}
			go func(line string) {
				defer func() { <-semaphore; done <- struct{}{} }()
				replaces := map[string]string{"ARG": line}
				for i, e := range strings.Split(line, *sep) {
					replaces[fmt.Sprintf("COL%d", i+1)] = e
				}
				script := strings.Join(slices.Clone(flag.Args()), " ")
				for k, v := range replaces {
					script = strings.ReplaceAll(script, k, v)
				}
				out, _ := exec.CommandContext(ctx, "bash", "-c", script).CombinedOutput()
				results <- out
			}(scanner.Text())
		}
		if scanner.Err() != nil {
			panic(scanner.Err())
		}
		for i := 0; i < count; i++ {
			<-done
		}
		close(results)
	}()

	for rr := range results {
		fmt.Print(string(rr))
	}
	return nil
}
