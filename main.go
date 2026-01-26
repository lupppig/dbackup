package main

import (
	"fmt"
	"os"

	"github.com/lupppig/dbackup/cmd"
)

const (
	EXIT_SUCCESS = iota
	EXIT_FAILURE
)

func main() {
	if err := cmd.Execute(); err != nil {
		exitOnError(err)
	}

	os.Exit(EXIT_SUCCESS)
}

func exitOnError(err error) {
	if err == nil {
		return
	}

	fmt.Fprintf(os.Stderr, fmt.Sprintf("error exit on %v", err))
	os.Exit(EXIT_FAILURE)
}
