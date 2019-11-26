// +build ignore

package main

import (
	"github.com/stalehd/clusterfunk/cmd/demo/server/http"
	"log"

	"github.com/shurcooL/vfsgen"
)

func main() {
	err := vfsgen.Generate(http.Assets, vfsgen.Options{
		PackageName:  "http",
		VariableName: "Assets",
	})
	if err != nil {
		log.Fatal(err)
	}
}
