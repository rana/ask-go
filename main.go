package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/rana/ask/cmd"
	"github.com/rana/ask/internal/version"
)

func main() {
	// Handle --version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version.Short())
		os.Exit(0)
	}

	cli := cmd.CLI{}
	ctx := kong.Parse(&cli,
		kong.Name("ask"),
		kong.Description("A thought preservation and amplification system"),
		kong.UsageOnError(),
	)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
