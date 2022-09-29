// Copyright 2021 The ChromiumOS Authors
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
// # Features
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
//	cmd := testexec.CommandContext(ctx, "some", "external", "command")
//	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
//	    return err
//	}
package testexec

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	tastexec "chromiumos/tast/exec"
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

	// doneFlg is set to 1, when the process is terminated but before collecting
	// the process. This can be accessed from various goroutines concurrently,
	// so it should be read/written through done() and setDone().
	doneFlg uint32

	// sigMu is the mutex lock to guard from sending signals to processes
	// which is already collected.
	sigMu sync.RWMutex
}

// RunOption is enum of options which can be passed to Run, Output,
// CombinedOutput and Wait to control precise behavior of them.
type RunOption = tastexec.RunOption

// DumpLogOnError is an option to dump logs if the executed command fails
// (i.e., exited with non-zero status code).
const DumpLogOnError = tastexec.DumpLogOnError

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

// SeparatedOutput runs an external command, waits for its completion and
// returns stdout/stderr output of the command separately.
func (c *Cmd) SeparatedOutput(opts ...RunOption) (stdout, stderr []byte, err error) {
	if c.Stdout != nil {
		return nil, nil, errStdoutSet
	}
	if c.Stderr != nil {
		return nil, nil, errStderrSet
	}

	var outbuf, errbuf bytes.Buffer
	c.Stdout = &outbuf
	c.Stderr = &errbuf

	if err := c.Start(); err != nil {
		return nil, nil, err
	}

	err = c.Wait(opts...)
	if err != nil {
		err = errors.Wrapf(err, "command %q returned non-zero error code", strings.Join(c.Args, " "))
	}

	return outbuf.Bytes(), errbuf.Bytes(), err
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
func (c *Cmd) Wait(opts ...RunOption) error {
	if c.Process == nil {
		return errNotStarted
	}
	if c.ProcessState != nil {
		return errAlreadyWaited
	}

	// Wait for the process to be terminated, without collecting the
	// process itself.
	if err := c.blockUntilWaitable(); err != nil {
		return err
	}

	// Instead of simple mutex and bool, here atomic variable and
	// R/W Lock is used for better performance and consistency with
	// standard library implementation.

	// Marking done atomically. At anytime after this point, sending
	// a signal will be guarded.
	c.setDone()
	// Stop the timeout watchdog here, because the process is terminated.
	c.watchdogStop <- true

	// Take and release the sigMu in order to wait for the completion
	// of signal sending which is already in process.
	c.sigMu.Lock()
	c.sigMu.Unlock()

	// Actual wait to collect the subprocess.
	werr := c.Cmd.Wait()
	cerr := c.ctx.Err()

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

// blockUntilWaitable waits for the process to be ready to be collected.
func (c *Cmd) blockUntilWaitable() error {
	for {
		// Use the same strategy with os/wait_waitid.go.
		var siginfo [16]uint64
		const P_PID = 1 // Taken from syscall. NOLINT
		_, _, err := unix.Syscall6(unix.SYS_WAITID, P_PID, uintptr(c.Process.Pid), uintptr(unsafe.Pointer(&siginfo)), unix.WEXITED|unix.WNOWAIT, 0, 0)
		if err == 0 {
			return nil
		}
		if err != unix.EINTR {
			return err
		}
	}
}

// setDone marks this command is already terminated.
// This and done below can be called from various goroutines concurrently.
func (c *Cmd) setDone() {
	atomic.StoreUint32(&c.doneFlg, 1)
}

// done returns true if this command is marked as terminated already.
// It is done in Wait(). This can be called from various goroutines
// concurrently.
func (c *Cmd) done() bool {
	return atomic.LoadUint32(&c.doneFlg) > 0
}

// Signal sends the input signal to the process tree.
//
// This is a new method that does not exist in os/exec.
//
// Even after successful completion of this function, you still need to call
// Wait to release all associated resources.
func (c *Cmd) Signal(signal unix.Signal) error {
	if c.Process == nil {
		return errNotStarted
	}

	// ProcessState may be set in another go-routine calling Wait(),
	// so there's a room of timing issue.
	// However, because we do not check the contents, also, "done()"
	// is checked below with sync mechanism, there should be not
	// a problem.
	if c.ProcessState != nil {
		return errAlreadyWaited
	}

	// Guard by a lock so that the signal won't be sent to the process
	// which is already terminated.
	c.sigMu.RLock()
	defer c.sigMu.RUnlock()
	if c.done() {
		return errAlreadyWaited
	}

	// Negative PID means the process group led by the process.
	return unix.Kill(-c.Process.Pid, signal)
}

// Kill sends SIGKILL to the process tree.
//
// This is a new method that does not exist in os/exec.
//
// Even after successful completion of this function, you still need to call
// Wait to release all associated resources.
func (c *Cmd) Kill() error {
	return c.Signal(unix.SIGKILL)
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
	var errExit *exec.ExitError
	ok = errors.As(err, &errExit)
	if !ok {
		return 0, false
	}
	status, ok = errExit.Sys().(syscall.WaitStatus)
	return status, ok
}

// ExitCode extracts exit code from error returned by exec.Command.Run().
// Returns exit code and true if exit code is extracted. (0, false) otherwise.
// Note that "true" does not mean that the process itself exited correctly, only
// that the exit code was extracted successfully.
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
