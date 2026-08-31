package command

import "errors"

type Command struct {
	Program string
	Args    []string
}

var ErrCommandNotAllowed = errors.New(
	"command is not allowed",
)
