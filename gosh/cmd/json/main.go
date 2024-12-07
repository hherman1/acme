package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	out := make(map[string]string)
	if (len(os.Args)-1)%2 != 0 {
		fmt.Println("invalid number of arguments")
		os.Exit(1)
	}
	for i := range os.Args[1:] {
		if i%2 == 0 {
			out[os.Args[i+1]] = os.Args[i+2]
		}
	}
	bs, err := json.Marshal(out)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(1)
	}
	fmt.Print(string(bs))
}
