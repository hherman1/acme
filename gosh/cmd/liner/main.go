package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/hherman1/acme/gosh"
)

func main() {
	root := strings.TrimSpace(S("git rev-parse --show-toplevel"))
	FailOnError = false
	paths := strings.Split(strings.TrimSpace(S("gh pr diff --name-only ")), "\n")
	FailOnError = true
	if LastErr != nil {
		paths = strings.Split(strings.TrimSpace(S("git diff --name-only")), "\n")
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		path := filepath.Join(root, p)
		FailOnError = false
		contents := Cat(path)
		FailOnError = true
		if os.IsNotExist(LastErr) {
			continue
		}
		if LastErr != nil {
			fmt.Printf("cat: %v\n", LastErr)
			continue
		}
		if strings.HasSuffix(contents, "\n") {
			continue
		}
		WriteFile(path, contents+"\n")
		fmt.Println("Added end newline: ", path)
	}
}
