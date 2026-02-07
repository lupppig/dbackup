package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lupppig/dbackup/cmd"
	apperrors "github.com/lupppig/dbackup/internal/errors"
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

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		l.Error(fmt.Sprintf("[%s] %s", appErr.Type, appErr.Message), "error", appErr.Err)
		if appErr.Hint != "" {
			fmt.Fprintf(os.Stderr, "\n💡 HINT: %s\n\n", appErr.Hint)
		}
	} else {
		l.Error("Command failed", "error", err)
	}

	os.Exit(EXIT_FAILURE)
}
