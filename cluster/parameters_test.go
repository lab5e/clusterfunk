package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParameters(t *testing.T) {
	assert := require.New(t)

	p := Parameters{Verbose: true}
	assert.NotPanics(func() { p.Final() })
}
