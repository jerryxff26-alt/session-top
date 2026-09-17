package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jerryxff26-alt/session-top/internal/cli"
	"github.com/jerryxff26-alt/session-top/internal/codex"
)

func main() {
	home, err := codex.HomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cli.Run(os.Stdout, os.Args[1:], home, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
