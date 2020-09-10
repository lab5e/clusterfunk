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
	"time"

	"github.com/sirupsen/logrus"
)

// TimeCall times the call and prints out the execution time in milliseconds in
// the go standard logrus.
func TimeCall(call func(), description string) {
	start := time.Now()
	call()
	diff := time.Since(start)
	logrus.Infof("%s took %f ms to execute", description, float64(diff)/float64(time.Millisecond))
}
