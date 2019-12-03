package toolbox
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
