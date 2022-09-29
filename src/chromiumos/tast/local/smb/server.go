// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package smb

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/testing"
)

// Server is the instance of the smb daemon.
type Server struct {
	cmd       *testexec.Cmd
	running   bool
	serverErr chan error
}

const (
	smbdServerBinaryPath       = "/usr/local/sbin/smbd"
	smbdServerTerminatedSignal = "signal: terminated"
)

// NewServer sets up a smb daemon using the supplied smb.conf file.
func NewServer(smbConf string) *Server {
	cmd := testexec.CommandContext(
		context.Background(), smbdServerBinaryPath, // NOLINT
		"--daemon",              // Start smbd as a daemon.
		"--foreground",          // Foreground the process, ensuring we can signal it via os.Signal.
		"--debug-stdout",        // Send the logs to stdout to ensure we can dump them on failure.
		"--no-process-group",    // Stop smbd from creating a process group.
		"--configfile="+smbConf, // Pass our custom smbd.conf file.
		"--debuglevel=5")        // Up the logging level to provide for better debugging.
	return &Server{cmd: cmd, running: false, serverErr: make(chan error)}
}

// Stop tries to gracefully shut down the underlying smb daemon by sending a
// SIGTERM signal to the process.
// https://www.samba.org/samba/docs/current/man-html/smbd.8.html
func (s *Server) Stop(ctx context.Context) error {
	if !s.running {
		serverErr := <-s.serverErr
		return errors.Wrap(serverErr, "failed to stop smbd, not running may have crashed")
	}

	// Reserve 5s to force kill smbd if we can't gracefully shut it down.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Attempt to send a SIGTERM to smbd.
	if err := s.cmd.Signal(unix.SIGTERM); err != nil {
		return errors.Wrap(err, "failed to send SIGTERM to smbd")
	}

	// If the shortened context hits the deadline, send a SIGKILL otherwise
	// recover the error (if any) after sending SIGTERM.
	select {
	case <-ctx.Done():
		s.cmd.Kill()
		return errors.New("failed trying to stop smbd, send SIGKILL instead")
	case err := <-s.serverErr:
		if err != nil {
			return errors.Wrap(err, "failed trying to stop smbd")
		}
	}
	return nil
}

// Start begins the smb daemon and ensures it's log file is adequately flushed
// to a file in the event an error occurs.
// A SIGTERM is not considered worth of a log dump here due to Stop() sending
// a SIGTERM to gracefully shut down the process.
func (s *Server) Start(ctx context.Context) error {
	if s.running {
		return errors.New("smbd already running")
	}
	if err := terminateRunningSmbdInstances(ctx); err != nil {
		return err
	}
	s.running = true
	go func() {
		output, err := s.cmd.CombinedOutput()
		s.running = false
		if err == nil {
			s.serverErr <- nil
			return
		}
		if err != nil && strings.Contains(err.Error(), smbdServerTerminatedSignal) {
			testing.ContextLog(ctx, "smbd received a terminated signal")
			s.serverErr <- nil
			return
		}
		testing.ContextLog(ctx, "smbd may have crashed, dumping logs: ", err)
		outDir, ok := testing.ContextOutDir(ctx)
		if ok {
			errorLogPath := filepath.Join(outDir, "smbd.log")
			if err := ioutil.WriteFile(errorLogPath, output, 0644); err != nil {
				testing.ContextLog(ctx, "Failed to write smbd logs to: ", errorLogPath)
			}
		} else {
			testing.ContextLog(ctx, "Failed to get the out directory to dump smbd logs")
		}
		s.serverErr <- errors.Wrap(err, "smbd has crashed")
	}()
	return nil
}

// terminateRunningSmbdInstances finds any running instances of smbd and
// terminates them prior to running to ensure uniqueness.
func terminateRunningSmbdInstances(ctx context.Context) (retErr error) {
	instances, err := procutil.FindAll(procutil.ByExe(smbdServerBinaryPath))
	if err != nil && err != procutil.ErrNotFound {
		return errors.Wrap(err, "failed to find running smbd instances")
	}
	if len(instances) == 0 {
		return nil
	}
	testing.ContextLogf(ctx, "Found %d running smbd instances, terminating them", len(instances))
	for _, proc := range instances {
		if err = proc.SendSignal(unix.SIGTERM); err != nil {
			retErr = errors.Wrap(err, "failed to terminate smbd instance")
		}
	}
	return
}
