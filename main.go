package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupting...")
		cancel()
		// Second signal forces exit
		<-sigChan
		fmt.Println("\nForce exiting...")
		os.Exit(1)
	}()

	cli := cmd.CLI{}
	kongCtx := kong.Parse(&cli,
		kong.Name("ask"),
		kong.Description("A thought preservation and amplification system"),
		kong.UsageOnError(),
	)

	// Bind the context for commands to use
	kongCtx.Bind(ctx)

	err := kongCtx.Run(&cmd.Context{Context: ctx})
	kongCtx.FatalIfErrorf(err)
}
