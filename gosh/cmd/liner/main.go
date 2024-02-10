package main

import (
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/hherman1/acme/gosh"
)

func main() {
	root := strings.TrimSpace(S("git rev-parse --show-toplevel"))
	paths := strings.Split(strings.TrimSpace(S("git diff --name-only")), "\n")
	for _, p := range paths {
		if p == "" {
			continue
		}
		path := filepath.Join(root, p)
		contents := Cat(path)
		if strings.HasSuffix(contents, "\n") {
			continue
		}
		WriteFile(path, contents+"\n")
		fmt.Println("Added end newline: ", path)
	}
}
