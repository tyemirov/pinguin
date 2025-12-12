package main

import (
	"fmt"
	"os"

	"github.com/tyemirov/pinguin/cmd/client/internal/command"
)

func main() {
	root := command.NewRootCommand(command.Dependencies{})
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)

	if execErr := root.Execute(); execErr != nil {
		fmt.Fprintln(os.Stderr, execErr)
		os.Exit(1)
	}
}
