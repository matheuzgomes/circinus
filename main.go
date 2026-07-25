package main

import (
	"fmt"
	"os"

	"circinus/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "circinus:", err)
		os.Exit(1)
	}
}
