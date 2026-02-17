package main

import (
	"fmt"
	"os"
)

func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		if exitErr, ok := err.(exitError); ok {
			if !exitErr.silent && exitErr.message != "" {
				fmt.Fprintln(os.Stderr, exitErr.message)
			}
			os.Exit(exitErr.code)
		}
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
