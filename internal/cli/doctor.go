package cli

import (
	"fmt"
	"io"
	"strings"
)

type doctorCheck struct {
	Name   string
	Passed bool
	Detail string
}

func (c Command) runDoctor(args []string) int {
	if isHelpOnly(args) {
		c.printDoctorHelp(c.Out)
		return 0
	}
	if len(args) != 0 {
		_, _ = fmt.Fprintf(c.Err, "goark doctor 不接受参数: %s\n", strings.Join(args, " "))
		return 2
	}
	checks := make([]doctorCheck, 0, 4)
	project, err := c.resolveProject(c.Dir, nil, nil, true)
	if err != nil {
		writeDoctorChecks(c.Out, []doctorCheck{{Name: "goark.build", Detail: err.Error()}})
		return 1
	}
	checks = append(checks, doctorCheck{Name: "goark.build", Passed: true, Detail: project.Root})
	if err := validateProjectTaskGraph(project); err != nil {
		checks = append(checks, doctorCheck{Name: "task graph", Detail: err.Error()})
	} else {
		checks = append(checks, doctorCheck{Name: "task graph", Passed: true, Detail: fmt.Sprintf("%d tasks", len(project.Build.Tasks))})
	}
	goVersion := c.captureGoVersion()
	checks = append(checks, doctorCheck{Name: "go toolchain", Passed: goVersion != "unavailable", Detail: goVersion})
	service, serviceErr := c.projectToolService()
	if serviceErr != nil {
		checks = append(checks, doctorCheck{Name: "tools", Detail: serviceErr.Error()})
	} else {
		for _, status := range service.Statuses(c.Context) {
			checks = append(checks, doctorCheck{
				Name: "tool " + status.Name, Passed: status.Status == "ready",
				Detail: status.Status + doctorDetailSuffix(status.Detail),
			})
		}
		if len(project.Build.Tools) == 0 {
			checks = append(checks, doctorCheck{Name: "tools", Passed: true, Detail: "0 tools"})
		}
	}
	writeDoctorChecks(c.Out, checks)
	for _, check := range checks {
		if !check.Passed {
			return 1
		}
	}
	return 0
}

func doctorDetailSuffix(detail string) string {
	if detail == "" {
		return ""
	}
	return ": " + detail
}

func writeDoctorChecks(writer io.Writer, checks []doctorCheck) {
	for _, check := range checks {
		status := "FAIL"
		if check.Passed {
			status = "PASS"
		}
		_, _ = fmt.Fprintf(writer, "%s %s: %s\n", status, check.Name, check.Detail)
	}
}

func (c Command) printDoctorHelp(writer io.Writer) {
	_, _ = fmt.Fprint(writer, "Usage:\n  goark doctor\n")
}
