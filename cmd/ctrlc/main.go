package main

//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"fmt"
	"os"

	"github.com/ExploratoryEngineering/clusterfunk/pkg/ctrlc"
	"github.com/alecthomas/kong"
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
