package cli

import "goark.dev/cli/internal/processrun"

type ProcessRequest = processrun.Request

type ProcessRunner = processrun.Runner

type osProcessRunner struct{}

func (osProcessRunner) Run(request ProcessRequest) error {
	return processrun.OSRunner{}.Run(request)
}
