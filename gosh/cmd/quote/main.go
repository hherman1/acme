package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Printf("%q", strings.Join(os.Args[1:], ""))
}
