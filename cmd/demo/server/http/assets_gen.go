// +build vfsgen

package main

import (
	"log"
	gohttp "net/http"

	"github.com/shurcooL/vfsgen"
)

func main() {
	err := vfsgen.Generate(gohttp.Dir("../html"), vfsgen.Options{
		PackageName:  "http",
		VariableName: "Assets",
		Filename:     "static_vfsdata.go",
	})
	if err != nil {
		log.Fatal(err)
	}
}
