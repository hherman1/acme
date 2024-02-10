package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/hherman1/acme/gosh"
)

func main() {
	if len(os.Args) != 3 {
		Failf("need 3 args\n")
	}
	FailOnError = false
	fn := filepath.Join("/tmp/", os.Args[2])
	epid := strings.TrimSpace(Cat(fn))
	if LastErr == nil {
		if epid != "" {
			S("ps -p", epid)
			if LastErr == nil {
				os.Exit(1)
			}
		}
		S("rm", fn)
	}
	WriteFile(fn, os.Args[1])
}
