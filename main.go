package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/tcard/sgo/sgo"
	"github.com/tcard/sgo/sgo/scanner"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Print(helpMsg)
		return
	}

	var buildFlags []string
	var extraArgs []string
	for i, arg := range os.Args[2:] {
		if arg[0] == '-' {
			buildFlags = append(buildFlags, arg)
		} else {
			extraArgs = os.Args[i+2:]
			break
		}
	}

	switch os.Args[1] {
	case "version":
		fmt.Println("sgo version 0.6 (compatible with go1.6)")
		return
	case "run":
		if len(extraArgs) == 0 {
			fmt.Fprintln(os.Stderr, "sgo run: no files listed")
			os.Exit(1)
		}
		created, errs := sgo.TranslateFilePaths(extraArgs...)
		reportErrs(errs...)
		if len(errs) > 0 {
			os.Exit(1)
		}
		runGoCommand("run", buildFlags, created...)
		return
	case "help":
		if len(extraArgs) == 0 {
			fmt.Print(helpMsg)
		} else {
			switch extraArgs[0] {
			case "translate":
				fmt.Print(translateHelpMsg)
				return
			case "version":
				fmt.Print(versionHelpMsg)
				return
			}
			runGoCommand("help", buildFlags, extraArgs...)
		}
		return
	case "translate":
		errs := sgo.TranslateFile(func() (io.Writer, error) { return os.Stdout, nil }, os.Stdin, "stdin.sgo")
		if len(errs) > 0 {
			reportErrs(errs...)
			os.Exit(1)
		}
		return
	}

	if len(extraArgs) == 0 {
		extraArgs = append(extraArgs, ".")
	}
	_, warnings, errs := sgo.TranslatePaths(extraArgs)
	reportErrs(warnings...)
	reportErrs(errs...)
	if len(errs) > 0 {
		os.Exit(1)
	}

	runGoCommand(os.Args[1], buildFlags, extraArgs...)
}

func reportErrs(errs ...error) {
	for _, err := range errs {
		if errs, ok := err.(scanner.ErrorList); ok {
			for _, err := range errs {
				fmt.Fprintln(os.Stderr, err)
			}
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func runGoCommand(cmd string, buildFlags []string, extraArgs ...string) {
	c := exec.Command("go", append(append([]string{cmd}, buildFlags...), extraArgs...)...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Run()
}

const helpMsg = `sgo is a tool for managing SGo source code.

Usage:

	sgo command [arguments]

All commands that the go tool supports are wrapped by the sgo tool. sgo will
translate all Go files to SGo affected before running them.

To see a list of those commands:

	go help

Additionally, SGo supports or overrides the following commands:
	
	translate   read SGo code, print the resulting Go code
	version     print SGo version, and the Go version it works with

Use "sgo help [command]" for more information about a command.

Use "go help" to see a complete list of help topics.
`

const translateHelpMsg = `usage: sgo translate

Translate reads SGo code from the standard input, and prints the resulting Go
code to the standard output.

If there are errors in the provided SGo code, they will be reported to the
standard error and the command will exit with a non-zero exit code.
`

const versionHelpMsg = `usage: sgo version

Version prints the SGo version. It also reports the Go version it is compatible
with. "Compatible" means that SGo compiles to this Go version, and is able to
import all the packages that this Go version is able to.
`
