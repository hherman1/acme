package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		return fmt.Errorf("expected 2-3 args, got %v", len(os.Args))
	}
	if len(os.Args) == 3 {
		err := os.Chdir(os.Args[2])
		if err != nil {
			return fmt.Errorf("cd %v: %w", os.Args[2], err)
		}
	}
	target := os.Args[1]
	
	pkg, strukt, method := parse(target)
	cmd := exec.Command("go", "doc", target)
	out := strings.Builder{}
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("run go doc: %w\n%v", err, out.String())
	}
	switch {
	case method !="":
		// no replacements needed on method docs
		fmt.Println(out.String())
	case strukt != "":
		// we looked up a struct, not a method, so we will need to replace strukt name ferences with qualified references so they can remain
		// interactive
		fmt.Println(strings.ReplaceAll(out.String(), strukt, fmt.Sprintf("%v.%v", pkg, strukt)))
	default:
		// Just a package. We need to prefix function and struct names with the package name so they can be clicked through.
		// Shorten the package name to whatever go doc calls it
		start := out.String()[len("package "):]
		end := strings.Index(start, " //")
		pkg = start[:end]
		pattern := regexp.MustCompile("(type|func) ([a-zA-Z]+)")
		fmt.Println(pattern.ReplaceAllString(out.String(), fmt.Sprintf("$1 %v.$2", pkg)))
	}
	return nil
}

func parse(target string) (pkg, strukt, method string) {
	// Default is that there is only a package
	pkg = target
	// Maybe we have a strukt
	i := strings.LastIndex(target, ".")
	if i == -1 {
		return
	}
	maybeStruct := target[i+1:]
	pattern := regexp.MustCompile("^[a-zA-Z]+$")
	if !pattern.MatchString(maybeStruct) {
		// Not a valid function of strukt name, so we must only have a package name
		return
	}
	strukt = maybeStruct
	pkg = target[:i]
	// Do we have a function?
	i = strings.LastIndex(pkg, ".")
	if i == -1 {
		return
	}
	maybeStruct = pkg[i+1:]
	if !pattern.MatchString(maybeStruct) {
		// Not a valid function or strukt name, so we must only have a package and strukt name
		return
	}
	// Two seemingly valid properties, so we seem to have pkg.strukt.method
	pkg = pkg[:i]
	method = strukt
	strukt = maybeStruct
	return
}