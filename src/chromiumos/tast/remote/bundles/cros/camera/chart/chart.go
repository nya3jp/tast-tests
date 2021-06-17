// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chart provides utility for displaying a chart on chart tablet paired
// with DUT in camerabox setup.
package chart

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// displayScript is the script installed on chart tablet for displaying chart.
const displayScript = "/usr/local/autotest/bin/display_chart.py"

// displayOutputLog is path on chart tablet placing logs from stdout/stderr of display chart script.
const displayOutputLog = "/tmp/chart_service.log"

// Chart displays chart files on the chart tablet in a camerabox setup.
type Chart struct {
	// conn is the SSH connection to the chart tablet.
	conn *ssh.Conn
	// dir is the directory saving all chart files on chart tablet.
	dir string
	// pid is the process id of running display chart script.
	pid string
}

// cleanup cleans up chart's (half-)initialized members and saves logs of chart process to |outDir|.
func cleanup(ctx context.Context, conn *ssh.Conn, dir, pid, outDir string) error {
	// The member initialization are chained in order of |conn|>|dir|>|pid|
	// with later one in the chain requiring previous one to be successfully
	// initialized first. So check later one exist can assume the previous
	// also exist.
	cleanupDisplayProcess := func(ctx context.Context, conn *ssh.Conn, pid, outDir string) (retErr error) {
		if len(pid) == 0 {
			return nil
		}
		defer func() {
			// Collect logs.
			if err := linuxssh.GetFile(ctx, conn, displayOutputLog, filepath.Join(outDir, filepath.Base(displayOutputLog)), linuxssh.PreserveSymlinks); err != nil {
				if retErr != nil {
					testing.ContextLogf(ctx, "Failed to pull chart script logs from %v: %v", displayOutputLog, err)
				} else {
					retErr = errors.Wrapf(err, "failed pull chart script logs from %v", displayOutputLog)
				}
			}
			if err := conn.Command("rm", displayOutputLog).Run(ctx); err != nil {
				if retErr != nil {
					testing.ContextLogf(ctx, "Failed to clean up %v on chart tablet: %v", displayOutputLog, err)
				} else {
					retErr = errors.Wrapf(err, "failed to clean up %v on chart tablet", displayOutputLog)
				}
			}
		}()

		if err := conn.Command("kill", "-2", pid).Run(ctx); err != nil {
			return errors.Wrap(err, "failed to send interrupt signal to close display script")
		}
		// Here we assume the script closing process should be very quick
		// and thus don't need to wait for its closing to end.

		return nil
	}
	cleanupFns := [](func() error){
		func() error { return cleanupDisplayProcess(ctx, conn, pid, outDir) },
		func() error {
			if len(dir) == 0 {
				return nil
			}
			if err := conn.Command("rm", "-rf", dir).Run(ctx); err != nil {
				return errors.Wrapf(err, "failed remove chart directory %v", dir)
			}
			return nil
		},
		func() error {
			if conn == nil {
				return nil
			}
			if err := conn.Close(ctx); err != nil {
				return errors.Wrap(err, "failed to close SSH connection to chart tablet")
			}
			return nil
		},
	}
	var firstErr error
	for _, cleanup := range cleanupFns {
		if err := cleanup(); err != nil {
			if firstErr == nil {
				firstErr = errors.Wrap(err, "failed to run cleanup process")
			} else {
				testing.ContextLog(ctx, "Failed to run cleanup process: ", err)
			}
		}
	}
	return firstErr
}

// connectChart dials SSH connection to chart tablet with the auth key of DUT.
func connectChart(ctx context.Context, d *dut.DUT, hostname string) (*ssh.Conn, error) {
	var sopt ssh.Options
	ssh.ParseTarget(hostname, &sopt)
	sopt.KeyDir = d.KeyDir()
	sopt.KeyFile = d.KeyFile()
	sopt.ConnectTimeout = 10 * time.Second
	return ssh.New(ctx, &sopt)
}

// New displays |chartPath| chart on either |altHostname| or |d|'s
// corresponding chart tablet and returns a new |Chart| instance.
func New(ctx context.Context, d *dut.DUT, altHostname, chartPath, outDir string) (_ *Chart, retErr error) {
	var conn *ssh.Conn
	var dir, pid string
	defer func() {
		if retErr != nil {
			if err := cleanup(ctx, conn, dir, pid, outDir); err != nil {
				testing.ContextLog(ctx, "Failed to cleanup: ", err)
			}
		}
	}()

	// Connect to chart tablet.
	if len(altHostname) > 0 {
		c, err := connectChart(ctx, d, altHostname)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to connect to chart with hostname %v", altHostname)
		}
		conn = c
	} else {
		c, err := d.DefaultCameraboxChart(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to chart with default '-tablet' suffix hostname")
		}
		conn = c
	}

	// Create temp directory for saving chart files.
	out, err := conn.Command("mktemp", "-d", "/tmp/chart_XXXXXX").Output(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create chart file directory on chart tablet")
	}
	dir = strings.TrimSpace(string(out))

	// Display chart on chart tablet.
	chartHostPath := filepath.Join(dir, filepath.Base(chartPath))
	if _, err := linuxssh.PutFiles(
		ctx, conn, map[string]string{chartPath: chartHostPath}, linuxssh.DereferenceSymlinks); err != nil {
		return nil, errors.Wrapf(err, "failed to send chart file in path %v to chart tablet", chartPath)
	}

	displayCmd := fmt.Sprintf(
		"(python2 %s %s > %s 2>&1) & echo -n $!",
		shutil.Escape(displayScript), shutil.Escape(chartHostPath), shutil.Escape(displayOutputLog))
	testing.ContextLog(ctx, "Start display chart process: ", displayCmd)
	out, err = conn.Command("sh", "-c", displayCmd).Output(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run display chart script")
	}
	pid = strings.TrimSpace(string(out))

	testing.ContextLog(ctx, "Poll for 'is ready' message for ensuring chart is ready")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		err := conn.Command("grep", "-q", "Chart is ready.", displayOutputLog).Run(ctx)
		switch err.(type) {
		case nil, *cryptossh.ExitError:
			// We reach here either when grep ready pattern succeed
			// with nil err returned or the pattern is not found
			// with ExitError returned.
			return err
		default:
			return testing.PollBreak(err)
		}
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for chart ready")
	}
	testing.ContextLog(ctx, "Display chart complete")

	return &Chart{conn, dir, pid}, nil
}

// Close closes the chart process and saves its logs to |outDir|.
func (c *Chart) Close(ctx context.Context, outDir string) error {
	if err := cleanup(ctx, c.conn, c.dir, c.pid, outDir); err != nil {
		return errors.Wrap(err, "failed to close chart")
	}
	return nil
}
