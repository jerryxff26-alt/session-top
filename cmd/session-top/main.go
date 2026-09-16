package main

import (
	"fmt"
	"os"
	"time"

	"github.com/session-top/session-top/internal/app"
	"github.com/session-top/session-top/internal/codex"
)

func main() {
	home, err := codex.HomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := app.Run(os.Stdout, os.Args[1:], home, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
