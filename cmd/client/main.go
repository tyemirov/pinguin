package main

import (
	"fmt"
	"io"
	"os"

	"github.com/tyemirov/pinguin/cmd/client/internal/command"
)

func main() {
	runAndExit(os.Args[1:], os.Stdout, os.Stderr, command.Dependencies{}, os.Exit)
}

func runAndExit(arguments []string, stdout io.Writer, stderr io.Writer, dependencies command.Dependencies, exit func(int)) {
	if exitCode := run(arguments, stdout, stderr, dependencies); exitCode != 0 {
		exit(exitCode)
	}
}

func run(arguments []string, stdout io.Writer, stderr io.Writer, dependencies command.Dependencies) int {
	root := command.NewRootCommand(dependencies)
	root.SetArgs(arguments)
	root.SetOut(stdout)
	root.SetErr(stderr)

	if execErr := root.Execute(); execErr != nil {
		fmt.Fprintln(stderr, execErr)
		return 1
	}
	return 0
}
