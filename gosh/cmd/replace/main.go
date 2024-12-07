package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: replace [patternfile] [replacementfile] [targetfile]")
	}
	pbs, err := os.ReadFile(os.Args[1])
	if errors.Is(err, os.ErrNotExist) {
		pbs = []byte(os.Args[1])
	} else if err != nil {
		return fmt.Errorf("read pattern file: %v", err)
	}
	pattern, err := regexp.Compile(string(pbs))
	if err != nil {
		return fmt.Errorf("compile pattern: %v", err)
	}
	rbs, err := os.ReadFile(os.Args[2])
	if errors.Is(err, os.ErrNotExist) {
		rbs = []byte(os.Args[2])
	} else if err != nil {
		return fmt.Errorf("read replacement file: %v", err)
	}
	tbs, err := os.ReadFile(os.Args[3])
	if err != nil {
		return fmt.Errorf("read target file: %v", err)
	}
	replaced := pattern.ReplaceAll(tbs, rbs)
	if err := os.WriteFile(os.Args[3], replaced, 0777); err != nil {
		return fmt.Errorf("write target file: %v", err)
	}
	return nil
}
