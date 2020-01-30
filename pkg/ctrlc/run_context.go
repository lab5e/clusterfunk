package ctrlc

// RunContext is the context passed on to the subcommands. Override this when
// reusing the commands in other projects.
type RunContext interface {
	ClusterServer() ManagementServerParameters
	ClusterCommands() CommandList
}

