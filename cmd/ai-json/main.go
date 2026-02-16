package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		help = flag.Bool("help", false, "show usage")
	)
	flag.Parse()

	if *help {
		printUsage()
		return
	}

	fmt.Fprintln(os.Stdout, "ai-json: CLI scaffold ready")
	fmt.Fprintln(os.Stdout, "next: event decoding, analytics, and reporting")
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "ai-json - advanced event stream analytics")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  go run ./cmd/ai-json [flags]")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Flags:")
	flag.PrintDefaults()
}
