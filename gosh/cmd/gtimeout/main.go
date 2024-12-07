package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

var duration = flag.Duration("d", 10*time.Second, "timeout for each command [default 10s]")

func main() {
	if err := run(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			_, _ = fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		if osErr, ok := err.(*exec.ExitError); ok {
			os.Exit(osErr.ExitCode())
		} else {
			_, _ = fmt.Fprintln(os.Stderr, err)
			flag.Usage()
			os.Exit(1)
		}
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()
	flag.Parse()
	if len(flag.Args()) == 0 {
		return fmt.Errorf("no command given")
	}
	ctx, cancel = context.WithTimeout(ctx, *duration)
	defer cancel()
	cmd := exec.CommandContext(ctx, flag.Args()[0], flag.Args()[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	go func() {
		<-ctx.Done()
		cmd.Process.Kill()
	}
	err := cmd.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("timeout: %w", ctx.Err())
	}
	return err
}
