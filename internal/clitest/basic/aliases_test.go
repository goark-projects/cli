package clitest

import "goark.dev/cli/internal/cli"

type Command = cli.Command

type ProcessRequest = cli.ProcessRequest

var Main = cli.Main

type recordingProcessRunner struct {
	requests []ProcessRequest
	err      error
}

func (r *recordingProcessRunner) Run(request ProcessRequest) error {
	r.requests = append(r.requests, request)
	return r.err
}
