package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"garmin-fit-to-json/internal/fitjson"
)

var version = "dev"

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("fit2json", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	showVersion := fs.Bool("version", false, "print version")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "Usage: fit2json input.fit output.json")
		return 2
	}

	if *showVersion {
		if fs.NArg() != 0 {
			fmt.Fprintln(stderr, "Usage: fit2json input.fit output.json")
			return 2
		}
		fmt.Fprintln(stdout, version)
		return 0
	}

	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "Usage: fit2json input.fit output.json")
		return 2
	}

	if err := fitjson.ConvertFile(fs.Arg(0), fs.Arg(1)); err != nil {
		fmt.Fprintf(stderr, "fit2json: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
