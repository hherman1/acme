package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	. "github.com/hherman1/acme/gosh"
)

func main() {
	if len(os.Args) < 3 {
		Failf("needs at least 2 arguments\n")
	}
	id := strings.TrimSpace(S("acmectl new"))
	pwd := os.Args[1] + "/"
	fmt.Println(pwd)
	fmt.Println("hello")
	S("acmectl ctl", id, "name", pwd+"+inpage")
	S("acmectl ctl", id, "nomenu")
	r, w := io.Pipe()
	done := make(chan error)
	go func() {
		cmd := exec.Command("acmectl", "write", id, "data")
		cmd.Stdin = r
		done <- cmd.Run()
	}()
	fmt.Println(os.Args)
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Dir = pwd
	cmd.Stdout = w
	cmd.Stderr = w
	err := cmd.Run()
	if err != nil {
		Failf("run: %v\n", err)
	}
	w.Close()
	err = <-done
	if err != nil {
		Failf("write to page: %v\n", err)
	}
	S("acmectl ctl", id, "clean")
	S("printf 0,0 | acmectl write", id, "addr")
	S("acmectl ctl", id, "dot=addr")
	S("acmectl ctl", id, "show")
}
