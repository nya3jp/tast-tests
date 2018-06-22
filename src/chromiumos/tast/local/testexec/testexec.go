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
//  if err := cmd.Run(); err != nil {
//      cmd.DumpLog(ctx)
//      return err
//  }
package testexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"chromiumos/tast/testing"
)

// Cmd represents an external command being prepared or run.
//
// This struct embeds Cmd in os/exec.
type Cmd struct {
	// Cmd is the underlying exec.Cmd object.
	*exec.Cmd

	// log is the buffer uncaptured stdout/stderr is sent to by default.
	log bytes.Buffer

	// ctx is the context given to Command that specifies the timeout of the
	// external command.
	ctx context.Context

	// watchdogStop is notified in Wait to ask the watchdog goroutine to stop.
	watchdogStop chan bool
}

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
func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Output runs an external command, waits for its completion and returns
// stdout output of the command.
//
// See os/exec package for details.
func (c *Cmd) Output() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errStdoutSet
	}

	var buf bytes.Buffer
	c.Stdout = &buf

	if err := c.Start(); err != nil {
		return nil, err
	}

	err := c.Wait()
	return buf.Bytes(), err
}

// CombinedOutput runs an external command, waits for its completion and
// returns stdout/stderr output of the command.
//
// See os/exec package for details.
func (c *Cmd) CombinedOutput() ([]byte, error) {
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

	err := c.Wait()
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

// Wait waits for the process to finish and releases all associated resources.
//
// See os/exec package for details.
func (c *Cmd) Wait() error {
	if c.Process == nil {
		return errNotStarted
	}
	if c.ProcessState != nil {
		return errAlreadyWaited
	}

	werr := c.Cmd.Wait()
	cerr := c.ctx.Err()

	c.watchdogStop <- true

	if cerr != nil {
		return cerr
	}
	return werr
}

// Kill sends SIGKILL to the process tree.
//
// This is a new method that does not exist in os/exec.
//
// Even after successful completion of this function, you still need to call
// Wait to release all associated resources.
func (c *Cmd) Kill() error {
	if c.Process == nil {
		return errNotStarted
	}
	if c.ProcessState != nil {
		return errAlreadyWaited
	}

	// Negative PID means the process group led by the process.
	return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
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
	} else {
		testing.ContextLog(ctx, "External command failed: ", c.ProcessState)
	}
	testing.ContextLog(ctx, "Command: ", ShellEscapeArray(c.Args))
	testing.ContextLog(ctx, "Dir: ", c.Dir)
	testing.ContextLog(ctx, "Uncaptured output:\n", c.log.String())
	return nil
}

var shellSafeRE = regexp.MustCompile(`^[A-Za-z0-9@%_+=:,./-]$`)

// ShellEscape escapes a string for shell commands.
func ShellEscape(s string) string {
	if shellSafeRE.MatchString(s) {
		return s
	}
	return "'" + strings.Replace(s, "'", `'"'"'`, -1) + "'"
}

// ShellEscapeArray escapes an array of strings for shell commands.
func ShellEscapeArray(args []string) string {
	escaped := make([]string, len(args))
	for i, arg := range args {
		escaped[i] = ShellEscape(arg)
	}
	return strings.Join(escaped, " ")
}
