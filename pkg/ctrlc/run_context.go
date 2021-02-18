package ctrlc

import "github.com/lab5e/clusterfunk/pkg/clientfunk"

// RunContext is the context passed on to the subcommands. Override this when
// reusing the commands in other projects.
type RunContext interface {
	ClientParams() clientfunk.ClientParameters
	ClusterCommands() CommandList
}
