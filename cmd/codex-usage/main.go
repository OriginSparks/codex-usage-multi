package main

import (
	"fmt"
	"os"

	"github.com/OriginSparks/codex-usage-multi/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
