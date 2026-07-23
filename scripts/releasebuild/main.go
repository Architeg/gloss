package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Architeg/gloss/internal/release"
)

func main() {
	version := flag.String("version", "", "stable release tag (vMAJOR.MINOR.PATCH)")
	output := flag.String("output", "", "existing empty output directory")
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "releasebuild: unexpected positional arguments")
		os.Exit(2)
	}
	if err := release.Build(context.Background(), *version, *output, *root); err != nil {
		fmt.Fprintf(os.Stderr, "releasebuild: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Validated Gloss %s release artifacts in %s\n", *version, *output)
	for _, target := range release.Targets() {
		fmt.Printf("  %s\n", target.Archive)
	}
	fmt.Println("  checksums.txt")
}
