package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type prependWriter struct {
	w       io.Writer
	buf     bytes.Buffer
	prepend string
}

func (p *prependWriter) Write(bs []byte) (n int, err error) {
	n, _ = p.buf.Write(bs)
	index := bytes.IndexByte(p.buf.Bytes(), '\n')
	if index == -1 {
		return n, nil
	}
	_, err = p.w.Write([]byte(p.prepend))
	if err != nil {
		return n, err
	}
	_, err = io.CopyN(p.w, &p.buf, int64(index+1))
	if err != nil {
		return n, err
	}
	return n, nil
}

func main() {
	if len(os.Args) < 2 {
		_, _ = fmt.Fprintln(os.Stderr, "no command given")
		os.Exit(1)
	}
	prepend := os.Args[1] + ": "
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdout = &prependWriter{prepend: prepend, w: os.Stdout}
	cmd.Stdin = os.Stdin
	cmd.Stderr = &prependWriter{prepend: prepend, w: os.Stderr}
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
}
