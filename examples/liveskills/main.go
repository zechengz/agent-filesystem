package main

import (
	"os"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	os.Exit(runCLI(os.Args[1:], cwd, realEnv(), os.Stdout, os.Stderr))
}
