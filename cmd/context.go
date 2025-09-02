package cmd

import "context"

// Context wraps context for command execution
type Context struct {
	context.Context
}
