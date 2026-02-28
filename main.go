package main

import (
	"fmt"
	"os"

	"github.com/rogeecn/iflow-go/cmd"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	return cmd.Execute()
}
