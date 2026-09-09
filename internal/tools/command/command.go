package command

import "errors"

type Command struct {
	Program string
	Args    []string
	// Dir is the working directory to run the command in — empty means
	// the executing process's own cwd, same as exec.Cmd's own default.
	Dir string
}

var ErrCommandNotAllowed = errors.New(
	"command is not allowed",
)
