package command

import (
	"context"
	"os/exec"
)

type Executor interface {
	Execute(
		ctx context.Context,
		command Command,
	) (string, error)
}

type LocalExecutor struct {
	policy Policy
}

func NewLocalExecutor(policy Policy) *LocalExecutor {
	return &LocalExecutor{
		policy: policy,
	}
}

func (e *LocalExecutor) Execute(
	ctx context.Context,
	command Command,
) (string, error) {

	decision := e.policy.Evaluate(command)

	if !decision.Allowed {
		return decision.Reason, ErrCommandNotAllowed
	}

	cmd := exec.CommandContext(
		ctx,
		command.Program,
		command.Args...,
	)

	output, err := cmd.CombinedOutput()

	return string(output), err
}
