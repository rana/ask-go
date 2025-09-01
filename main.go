package main

import (
	"github.com/alecthomas/kong"
	"github.com/rana/ask/cmd"
)

func main() {
	cli := cmd.CLI{}
	ctx := kong.Parse(&cli,
		kong.Name("ask"),
		kong.Description("A thought preservation and amplification system"),
		kong.UsageOnError(),
	)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}