package ctrlc

// RunContext is the context passed on to the subcommands. Override this when
// reusing the commands in other projects.
type RunContext interface {
	ServerParameters() ManagementServerParameters
	Commands() CommandList
}

// NewRunContext creates a new RunContext from the command
func NewRunContext(cmd Parameters) RunContext {
	return &internalRunContext{params: cmd}
}

type internalRunContext struct {
	params Parameters
}

func (i *internalRunContext) ServerParameters() ManagementServerParameters {
	return i.params.Server
}

func (i *internalRunContext) Commands() CommandList {
	return i.params.Commands
}
