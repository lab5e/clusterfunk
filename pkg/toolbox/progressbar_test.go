package toolbox

import (
	"testing"
)

func TestProgressBar(t *testing.T) {
	tableTest := func(max int) {
		p := ConsoleProgress{Max: max}

		for i := 0; i < max+1; i++ {
			p.Print(i)
		}
	}

	tableTest(1)
	tableTest(50)
	tableTest(77)
	tableTest(100)
	tableTest(500)
}
