package toolbox

import (
	"fmt"
	"strings"
)

const progressWidth = 80

// ConsoleProgress prints an 80-character progress bar on the console for values
// between [0, Max]
type ConsoleProgress struct {
	Max     int
	Current int
}

// Print prints a 80-character wide progress bar on the console.
func (c *ConsoleProgress) Print(val int) {
	if val > c.Max {
		val = c.Max
	}
	pos := (float64(val) / float64(c.Max)) * float64(progressWidth-2)

	newVal := int(pos)
	if newVal == c.Current {
		// nothing to print since there's no change
		return
	}
	c.Current = newVal

	str := []byte(fmt.Sprintf("[%s]", strings.Repeat(" ", progressWidth-2)))
	str[0] = byte('[')
	str[progressWidth-1] = ']'

	for i := 1; i <= c.Current; i++ {
		str[i] = '='
	}
	fmt.Printf("%s\r", string(str))

	if c.Current == (progressWidth - 2) {
		fmt.Println()
	}
}
