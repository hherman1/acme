package gosh

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var history []string

func S(args ...any) string {
	if len(args) == 0 {
		panic("no args in S")
	}
	var torun strings.Builder
	for i, a := range args {
		if i != 0 {
			torun.WriteRune(' ')
		}
		fmt.Fprintf(&torun, "%v", a)
	}
	history = append(history, torun.String())
	var output strings.Builder
	cmd := exec.Command("bash", "-c", torun.String())
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if err != nil {
		failf("%v\nexec: %v\n", output.String(), err)
	}
	return output.String()
}

func Cat(path string) string {
	history = append(history, "cat "+path)
	bs, err := os.ReadFile(path)
	if err != nil {
		failf("read file: %v", err)
	}
	return string(bs)
}

func WriteFile(path, contents string) {
	history = append(history, "write file: "+path)
	err := os.WriteFile(path, []byte(contents), 0777)
	if err != nil {
		failf("write file %v: %v", path, err)
	}
}

func failf(format string, args ...any) {
	for _, h := range history {
		fmt.Println("> ", h)
	}
	fmt.Printf(format, args...)
	os.Exit(1)
}
