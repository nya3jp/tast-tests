// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tcpdump

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/common/network/daemonutil"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Runner contains methods rely on running "tcpdump" command.
type Runner struct {
	cmd    cmd.Runner
	config *Config
}

// Config controls a tcpdump process.
type Config struct {
	iface      string
	packetPath string
	snaplen    uint64
	stdoutFile *os.File
	stderrFile *os.File
	wg         sync.WaitGroup
}

const (
	tcpdumpCmd               = "tcpdump"
	durationForClose         = 4 * time.Second
	durationForInternalClose = 2 * time.Second
)

// NewRunner creates an tcpdump command utility runner.
func NewRunner(cmd cmd.Runner) *Runner {
	return &Runner{
		cmd: cmd,
	}
}

// StartTcpdump executes tcpdump command.
// After getting a Runner instance, r, the caller should call r.Close() at the end,
// and use the shortened ctx (provided by r.ReserveForClose()) before r.Close() to
// reserve time for it to run.
func (r *Runner) StartTcpdump(ctx context.Context, iface, packetPath string, stdoutFile, stderrFile *os.File) error {
	r.config = &Config{
		iface:      iface,
		packetPath: packetPath,
		stdoutFile: stdoutFile,
		stderrFile: stderrFile,
	}

	if err := r.start(ctx); err != nil {
		return err
	}
	return nil
}

// start sets the command's standard output and creates a pipe to the command's
// standard error before executing the tcpdump command.
func (r *Runner) start(fullCtx context.Context) (err error) {
	// Clean up on error.
	defer func() {
		if err != nil {
			r.close(fullCtx)
		}
	}()

	// Reserve time for the above deferred call.
	ctx, ctxCancel := ctxutil.Shorten(fullCtx, durationForInternalClose)
	defer ctxCancel()

	testing.ContextLogf(ctx, "Starting tcpdump on %s", r.config.iface)

	args := []string{"-U", "-i", r.config.iface, "-w", r.config.packetPath}
	if r.config.snaplen != 0 {
		args = append(args, "-s", strconv.FormatUint(r.config.snaplen, 10))
	}

	r.cmd.CreateCmd(ctx, tcpdumpCmd, args...)

	r.cmd.SetStdOut(r.config.stdoutFile)

	stderrPipe, err := r.cmd.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StderrPipe of tcpdump")
	}
	readyFunc := func(buf []byte) (bool, error) {
		return bytes.Contains(buf, []byte("listening on")), nil
	}
	readyWriter := daemonutil.NewReadyWriter(readyFunc)
	r.config.wg.Add(1)
	go func() {
		defer r.config.wg.Done()
		defer stderrPipe.Close()
		defer readyWriter.Close()
		multiWriter := io.MultiWriter(r.config.stderrFile, readyWriter)
		io.Copy(multiWriter, stderrPipe)
	}()
	if err := r.cmd.StartCmd(); err != nil {
		return errors.Wrap(err, "failed to start tcpdump")
	}

	testing.ContextLog(ctx, "Waiting for tcpdump to be ready")
	readyCtx, readyCtxCancel := context.WithTimeout(ctx, 15*time.Second)
	defer readyCtxCancel()
	if err := readyWriter.Wait(readyCtx); err != nil {
		return err
	}

	// To avoid tcpdump process becoming a zombie process after its parent process terminates,
	// ReleaseProcess() is used to detach the tcpdump process from its parent process.
	// Remote commands do not require ReleaseProcess().
	err = r.cmd.ReleaseProcess()
	if err != nil {
		return errors.Wrap(err, "failed to release tcpdump process")
	}

	return nil
}

// SetSnaplen sets a tcpdump's snapshot length.
func (r *Runner) SetSnaplen(s uint64) {
	r.config.snaplen = s
}

// close kills the process, tries to releases occupied resources.
func (r *Runner) close(ctx context.Context) error {
	var err error
	if r.CmdExists() {
		// Kill with SIGTERM here, so that the process can flush buffer.
		// If the process does not die before deadline, cmd.Wait will then abort it.
		// TODO(crbug.com/1030635):
		// Signal through SSH might not work. Use pkill to send signal for now.
		r.cmd.Run(ctx, "pkill", "-f", fmt.Sprintf("^%s.*%s", tcpdumpCmd, r.config.packetPath))
		r.cmd.WaitCmd()
		r.cmd.ResetCmd()
	}
	r.config.wg.Wait()
	if r.config.stderrFile != nil {
		r.config.stderrFile.Close()
		r.config.stderrFile = nil
	}
	if r.config.stdoutFile != nil {
		r.config.stdoutFile.Close()
		r.config.stdoutFile = nil
	}
	return err
}

// ReserveForClose returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.Close() to reserve time for it to run.
func (r *Runner) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, durationForClose)
}

// Close terminates the tcpdump command.
func (r *Runner) Close(ctx context.Context) error {
	// Wait 2 seconds (2 * libpcap poll timeout) before killing the
	// process so that it can properly catch all packets.
	// Investigation of the timeout can be found in crrev.com/c/288814.
	// TODO(b/154787243): Find a better way to wait for pcap to be done.
	testing.Sleep(ctx, 2*time.Second)
	return r.close(ctx)
}

// Output returns packets captured by the tcpdump utility.
// args is set to filter tcpdump packets.
func (r *Runner) Output(ctx context.Context, args ...string) ([]byte, error) {
	args = append(args, "-r", r.config.packetPath)
	return r.cmd.Output(ctx, "tcpdump", args...)
}

// CmdExists confirms whether the tcpdump command has been successfully started.
func (r *Runner) CmdExists() bool {
	return r.cmd.CmdExists()
}
