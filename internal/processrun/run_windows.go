//go:build windows

package processrun

import (
	"context"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func run(ctx context.Context, command *exec.Cmd) error {
	job, err := createKillOnCloseJob()
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	if err := command.Start(); err != nil {
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return err
	}
	assignErr := windows.AssignProcessToJobObject(job, process)
	_ = windows.CloseHandle(process)
	if assignErr != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return assignErr
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	select {
	case err := <-waited:
		return err
	case <-ctx.Done():
		_ = windows.TerminateJobObject(job, 1)
		<-waited
		return ctx.Err()
	}
}

func createKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	information := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	information.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&information)),
		uint32(unsafe.Sizeof(information)),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}
