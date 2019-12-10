package clientfunk
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

	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk"
	"github.com/ExploratoryEngineering/clusterfunk/pkg/toolbox"
	"github.com/stretchr/testify/require"
)

func TestManagementZCLookup(t *testing.T) {
	assert := require.New(t)
	reg := toolbox.NewZeroconfRegistry("test")
	assert.NotNil(reg)

	assert.NoError(reg.Register(funk.ZeroconfManagementKind, "1", 1234))
	defer reg.Shutdown()

	s, err := ZeroconfManagementLookup("test")
	assert.NoError(err)
	assert.NotEmpty(s)
}
