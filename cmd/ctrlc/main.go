package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/lab5e/clusterfunk/pkg/ctrlc"
)

func main() {

	defer func() {
		// The Kong parser panics when there's a sole dash in the argument list
		// I'm not sure if this is a bug or a feature :)
		if r := recover(); r != nil {
			fmt.Println("Error parsing command line: ", r)
		}
	}()

	var param ctrlc.Parameters
	ctx := kong.Parse(&param,
		kong.Name("ctrlc"),
		kong.Description("Clusterfunk management CLI"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: true,
		}))
	// This binds the interface ctrlc.RunContext to the implementation we're
	// using.
	ctx.BindTo(&param, (*ctrlc.RunContext)(nil))
	if err := ctx.Run(); err != nil {
		// Won't print the error since the commands will do it
		os.Exit(1)
	}
}
