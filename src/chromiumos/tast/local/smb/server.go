// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package smb

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"syscall"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Server is the instance of the smb daemon.
type Server struct {
	cmd     *testexec.Cmd
	running bool
}

const (
	smbdServerBinaryPath       = "/usr/local/sbin/smbd"
	smbdServerTerminatedSignal = "signal: terminated"
)

// NewServer sets up a smb daemon using the supplied smb.conf file.
func NewServer(ctx context.Context, smbConf string) *Server {
	cmd := testexec.CommandContext(
		context.Background(), smbdServerBinaryPath, // NOLINT
		"--daemon",              // Start smbd as a daemon.
		"--foreground",          // Foreground the process, ensuring we can signal it via os.Signal.
		"--log-stdout",          // Send the logs to stdout to ensure we can dump them on failure.
		"--no-process-group",    // Stop smbd from creating a process group.
		"--configfile="+smbConf, // Pass our custom smbd.conf file.
		"--debuglevel=5")        // Up the logging level to provide for better debugging.
	return &Server{cmd, false}
}

// Stop tries to gracefully shut down the underlying smb daemon by sending a
// SIGTERM signal to the process.
// https://www.samba.org/samba/docs/current/man-html/smbd.8.html
func (s *Server) Stop(ctx context.Context) error {
	if !s.running {
		return errors.New("failed to stop smbd, not running may have crashed")
	}
	if err := s.cmd.Signal(syscall.SIGTERM); err != nil {
		return errors.Wrap(err, "failed to send SIGTERM to smbd")
	}
	testing.ContextLog(ctx, "Successfully sent SIGTERM to smbd")
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
	s.running = true
	output, err := s.cmd.CombinedOutput()
	s.running = false
	if err == nil {
		return nil
	}
	if err != nil && strings.Contains(fmt.Sprint(err), smbdServerTerminatedSignal) {
		testing.ContextLog(ctx, "smbd received a terminated signal")
		return nil
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
	return errors.New("smbd has crashed")
}
