package command

import "context"

type Executor interface {
	Execute(
		ctx context.Context,
		command Command,
	) (string, error)
}
