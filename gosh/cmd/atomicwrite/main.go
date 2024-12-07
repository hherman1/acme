package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

func main() {
	if len(os.Args) < 2 {
		_, _ = fmt.Fprintln(os.Stderr, "no file given")
		os.Exit(1)
	}
	f, err := os.Create(filepath.Join(os.TempDir(), uuid.NewString()))
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "create temp file:", err)
		os.Exit(1)
	}
	defer f.Close()
	_, err = io.Copy(f, os.Stdin)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "write temp file:", err)
		os.Exit(1)
	}
	err = f.Close()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "close temp file:", err)
		os.Exit(1)
	}
	err = os.Rename(f.Name(), os.Args[1])
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "rename temp file:", err)
		os.Exit(1)
	}
}
