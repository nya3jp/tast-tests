// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testexec is a wrapper of the standard os/exec package optimized for
// use cases of Tast. Tast tests should always use this package instead of
// os/exec.
//
// This package is designed to be a drop-in replacement of os/exec. Just
// rewriting imports should work. In addition, several methods are available,
// such as Kill and DumpLog.
//
// Features
//
// Automatic log collection. os/exec sends stdout/stderr to /dev/null unless
// explicitly specified to collect them. This default behavior makes it very
// difficult to debug external command failures. This wrapper automatically
// collects those uncaptured logs and allows to log them later.
//
// Process group handling. On timeout, os/exec kills the direct child process
// only. This can often leave orphaned subprocesses in DUT and interfere with
// later tests. To avoid this issue, this wrapper will kill the whole tree
// of subprocesses on timeout by setting process group ID appropriately.
//
// Usage
//
//  cmd := testexec.CommandContext(ctx, "some", "external", "command")
//  if err := cmd.Run(testexec.DumpLogOnError); err != nil {
//      return err
//  }
package testexec

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// Cmd represents an external command being prepared or run.
//
// This struct embeds Cmd in os/exec.
//
// Callers may wish to use shutil.EscapeSlice when logging Args.
type Cmd struct {
	// Cmd is the underlying exec.Cmd object.
	*exec.Cmd
	// log is the buffer uncaptured stdout/stderr is sent to by default.
	log bytes.Buffer
	// ctx is the context given to Command that specifies the timeout of the external command.
	ctx context.Context
	// timedOut indicates if the process hit timeout. This is set in Wait().
	timedOut bool
	// watchdogStop is notified in Wait to ask the watchdog goroutine to stop.
	watchdogStop chan bool
}

// RunOption is enum of options which can be passed to Run, Output,
// CombinedOutput and Wait to control precise behavior of them.
type RunOption int

// DumpLogOnError is an option to dump logs if the executed command fails
// (i.e., exited with non-zero status code).
const DumpLogOnError RunOption = iota

var (
	errStdoutSet      = errors.New("Stdout was already set")
	errStderrSet      = errors.New("Stderr was already set")
	errAlreadyStarted = errors.New("Start was already called")
	errNotStarted     = errors.New("Start was not yet called")
	errAlreadyWaited  = errors.New("Wait was already called")
	errNotWaited      = errors.New("Wait was not yet called")
)

// CommandContext prepares to run an external command.
//
// Timeout set in ctx is honored on running the command.
//
// See os/exec package for details.
func CommandContext(ctx context.Context, name string, arg ...string) *Cmd {
	cmd := exec.Command(name, arg...)

	// Enable Setpgid so we can terminate the whole subprocesses.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	return &Cmd{
		Cmd:          cmd,
		ctx:          ctx,
		watchdogStop: make(chan bool, 1),
	}
}

// Run runs an external command and waits for its completion.
//
// See os/exec package for details.
func (c *Cmd) Run(opts ...RunOption) error {
	if err := c.Start(); err != nil {
		return err
	}

	err := c.Wait(opts...)
	return err
}

// Output runs an external command, waits for its completion and returns
// stdout output of the command.
//
// See os/exec package for details.
func (c *Cmd) Output(opts ...RunOption) ([]byte, error) {
	if c.Stdout != nil {
		return nil, errStdoutSet
	}

	var buf bytes.Buffer
	c.Stdout = &buf

	if err := c.Start(); err != nil {
		return nil, err
	}

	err := c.Wait(opts...)
	return buf.Bytes(), err
}

// CombinedOutput runs an external command, waits for its completion and
// returns stdout/stderr output of the command.
//
// See os/exec package for details.
func (c *Cmd) CombinedOutput(opts ...RunOption) ([]byte, error) {
	if c.Stdout != nil {
		return nil, errStdoutSet
	}
	if c.Stderr != nil {
		return nil, errStderrSet
	}

	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf

	if err := c.Start(); err != nil {
		return nil, err
	}

	err := c.Wait(opts...)
	return buf.Bytes(), err
}

// Start starts an external command.
//
// See os/exec package for details.
func (c *Cmd) Start() error {
	if c.Process != nil {
		return errAlreadyStarted
	}

	// Return early if deadline is already expired.
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
	}

	// Collect stdout/stderr to log by default.
	if c.Stdout == nil {
		c.Stdout = &c.log
	}
	if c.Stderr == nil {
		c.Stderr = &c.log
	}

	if err := c.Cmd.Start(); err != nil {
		return err
	}

	// Watchdog goroutine to terminate the process on timeout.
	go func() {
		// TODO(nya): Avoid the race condition between reaping the child process
		// and sending a signal.
		select {
		case <-c.ctx.Done():
			c.Kill()
		case <-c.watchdogStop:
		}
	}()

	return nil
}

// logPipe writes each line from `pipe` to the test's logs.
func (c *Cmd) logPipe(pipe io.ReadCloser) {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		testing.ContextLog(c.ctx, scanner.Text())
	}
}

// LogStdout writes the stdout of the process to the test's logs.
func (c *Cmd) LogStdout() error {
	if c.Stdout != nil {
		return errStdoutSet
	}

	stdout, err := c.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe")
	}

	go c.logPipe(stdout)
	return nil
}

// LogStderr writes the stderr of the process to the test's logs.
func (c *Cmd) LogStderr() error {
	if c.Stderr != nil {
		return errStderrSet
	}

	stderr, err := c.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stderr pipe")
	}

	go c.logPipe(stderr)
	return nil
}

// Wait waits for the process to finish and releases all associated resources.
//
// See os/exec package for details.
func (c *Cmd) Wait(opts ...RunOption) error {
	if c.Process == nil {
		return errNotStarted
	}
	if c.ProcessState != nil {
		return errAlreadyWaited
	}

	werr := c.Cmd.Wait()
	cerr := c.ctx.Err()

	c.watchdogStop <- true

	if (werr != nil || cerr != nil) && hasOpt(DumpLogOnError, opts) {
		// Ignore the DumpLog intentionally, because the primary error
		// here is either werr or cerr. Note that, practically, the
		// error from DumpLog is returned when ProcessState is nil,
		// so it shouldn't happen here, because it should be assigned
		// in Wait() above.
		c.DumpLog(c.ctx)
	}

	if cerr != nil {
		c.timedOut = true
		return cerr
	}
	return werr
}

// Signal sends the input signal to the process tree.
//
// This is a new method that does not exist in os/exec.
//
// Even after successful completion of this function, you still need to call
// Wait to release all associated resources.
func (c *Cmd) Signal(signal syscall.Signal) error {
	if c.Process == nil {
		return errNotStarted
	}
	if c.ProcessState != nil {
		return errAlreadyWaited
	}

	// Negative PID means the process group led by the process.
	return syscall.Kill(-c.Process.Pid, signal)
}

// Kill sends SIGKILL to the process tree.
//
// This is a new method that does not exist in os/exec.
//
// Even after successful completion of this function, you still need to call
// Wait to release all associated resources.
func (c *Cmd) Kill() error {
	return c.Signal(syscall.SIGKILL)
}

// Cred is a helper function that sets SysProcAttr.Credential to control
// the credentials (e.g. UID, GID, etc.) used to run the command.
func (c *Cmd) Cred(cred syscall.Credential) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Credential = &cred
}

// DumpLog logs details of the executed external command, including uncaptured output.
//
// This is a new method that does not exist in os/exec.
//
// Call this function when the test is failing due to unexpected external command result.
// You should not call this function for every external command invocation to avoid
// spamming logs.
//
// This function must be called after Wait.
func (c *Cmd) DumpLog(ctx context.Context) error {
	if c.ProcessState == nil {
		return errNotWaited
	}

	if c.ProcessState.Success() {
		testing.ContextLog(ctx, "External command succeeded")
	} else if c.timedOut {
		testing.ContextLog(ctx, "External command timed out")
	} else {
		testing.ContextLog(ctx, "External command failed: ", c.ProcessState)
	}
	testing.ContextLog(ctx, "Command: ", shutil.EscapeSlice(c.Args))
	testing.ContextLog(ctx, "Uncaptured output:\n", c.log.String()) // NOLINT
	return nil
}

// GetWaitStatus extracts WaitStatus from error.
// WaitStatus is typically returned from Run, Output, CombinedOutput and Wait to
// indicate a child process's exit status.
// If err is nil, it returns WaitStatus representing successful exit.
func GetWaitStatus(err error) (status syscall.WaitStatus, ok bool) {
	if err == nil {
		return 0, true
	}
	errExit, ok := err.(*exec.ExitError)
	if !ok {
		return 0, false
	}
	status, ok = errExit.Sys().(syscall.WaitStatus)
	return status, ok
}

// ExitCode extracts exit code from error returned by exec.Command.Run().
// Returns exit code and true when succcess. (0, false) otherwise.
func ExitCode(cmdErr error) (int, bool) {
	s, ok := GetWaitStatus(cmdErr)
	if !ok {
		return 0, false
	}
	if s.Exited() {
		return s.ExitStatus(), true
	}
	if s.Signaled() {
		return int(s.Signal()) + 128, true
	}
	return 0, false
}

// hasOpt returns whether the given opts contain the opt.
func hasOpt(opt RunOption, opts []RunOption) bool {
	for _, o := range opts {
		if o == opt {
			return true
		}
	}
	return false
}
