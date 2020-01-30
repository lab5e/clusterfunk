package ctrlc

// RunContext is the context passed on to the subcommands. Override this when
// reusing the commands in other projects.
type RunContext interface {
	ClusterServer() ManagementServerParameters
	ClusterCommands() CommandList
}

// NewRunContext creates a new RunContext from the command
func NewRunContext(cmd Parameters) RunContext {
	return &internalRunContext{params: cmd}
}

type internalRunContext struct {
	params Parameters
}

func (i *internalRunContext) ClusterServer() ManagementServerParameters {
	return i.params.Server
}

func (i *internalRunContext) ClusterCommands() CommandList {
	return i.params.Commands
}
