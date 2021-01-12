// +build tools

package tools

import (
	_ "github.com/mgechev/revive"
	_ "golang.org/x/lint/golint"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
