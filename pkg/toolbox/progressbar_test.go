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
