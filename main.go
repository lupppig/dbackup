package main

import (
	"os"

	"github.com/lupppig/dbackup/cmd"
	"github.com/lupppig/dbackup/internal/logger"
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

	l := logger.New(logger.Config{})
	l.Error("Command failed", "error", err)
	os.Exit(EXIT_FAILURE)
}
