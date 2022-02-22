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
	"regexp"
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

// DisplayDefaultLevel presents the default display level
const DisplayDefaultLevel = -1

// Chart displays chart files on the chart tablet in a camerabox setup.
type Chart struct {
	// conn is the SSH connection to the chart tablet.
	conn *ssh.Conn
	// dir is the directory saving all chart files on chart tablet.
	dir string
	// pid is the process id of running display chart script.
	pid string
	// fifo is the path to fifo on chart tablet which can be used to write
	// display configuration into it.
	fifo string
}

// NamePath is the reference path to the chart to be displayed on the chart
// service. The path is determined by the relative path to the chart directory.
type NamePath string

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
			if err := conn.CommandContext(ctx, "rm", displayOutputLog).Run(); err != nil {
				if retErr != nil {
					testing.ContextLogf(ctx, "Failed to clean up %v on chart tablet: %v", displayOutputLog, err)
				} else {
					retErr = errors.Wrapf(err, "failed to clean up %v on chart tablet", displayOutputLog)
				}
			}
		}()

		if err := conn.CommandContext(ctx, "kill", "-2", pid).Run(); err != nil {
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
			if err := conn.CommandContext(ctx, "rm", "-rf", dir).Run(); err != nil {
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

// New starts chart service waiting for display chart command on the host
// either |altHostname| or |d|'s corresponding chart tablet, prepares all chart
// files to be displayed from |chartPaths| and returns a new |Chart| instance.
func New(ctx context.Context, d *dut.DUT, altHostname, outDir string, chartPaths []string) (*Chart, []NamePath, error) {
	var conn *ssh.Conn

	// Connect to chart tablet.
	if len(altHostname) > 0 {
		c, err := connectChart(ctx, d, altHostname)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to connect to chart with hostname %v", altHostname)
		}
		conn = c
	} else {
		c, err := d.DefaultCameraboxChart(ctx)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to connect to chart with default '-tablet' suffix hostname")
		}
		conn = c
	}

	return SetUp(ctx, conn, outDir, chartPaths)
}

// copyChart copies the chart from local to host chart tablet.
func copyChart(ctx context.Context, conn *ssh.Conn, chartLocalPath, chartHostDir string) error {
	chartHostPath := filepath.Join(chartHostDir, filepath.Base(chartLocalPath))
	if _, err := linuxssh.PutFiles(
		ctx, conn, map[string]string{chartLocalPath: chartHostPath}, linuxssh.DereferenceSymlinks); err != nil {
		return errors.Wrapf(err, "failed to send chart file in path %v to chart tablet", chartLocalPath)
	}
	return nil
}

// SetUp sets up the chart with the given ssh connection and returns a new |Chart| instance.
// It uses |displayLevel| to set the brightness, the range is in [0.0, 100.0].
func SetUp(ctx context.Context, conn *ssh.Conn, outDir string, chartPaths []string) (_ *Chart, _ []NamePath, retErr error) {
	var dir, pid string
	defer func() {
		if retErr != nil {
			if err := cleanup(ctx, conn, dir, pid, outDir); err != nil {
				testing.ContextLog(ctx, "Failed to cleanup: ", err)
			}
		}
	}()

	// Create temp directory for saving chart files.
	out, err := conn.CommandContext(ctx, "mktemp", "-d", "/tmp/chart_XXXXXX").Output()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create chart file directory on chart tablet")
	}
	dir = strings.TrimSpace(string(out))

	// Due to the limitation of telemetry |SetHTTPServerDirectories()|
	// which requires all files prepared before start the http server.
	// Copies all chart files into chart directory in setup stage.
	fileMap := make(map[string]string, 0)
	namePaths := make([]NamePath, 0)
	for _, localPath := range chartPaths {
		baseName := filepath.Base(localPath)
		chartHostPath := filepath.Join(dir, baseName)
		fileMap[localPath] = chartHostPath
		namePaths = append(namePaths, NamePath(baseName))
	}
	if _, err := linuxssh.PutFiles(ctx, conn, fileMap, linuxssh.DereferenceSymlinks); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to send chart files %v to chart tablet", chartPaths)
	}

	// Start chart service.
	displayCmd := fmt.Sprintf(
		"(python %s %s > %s 2>&1) & echo -n $!",
		shutil.Escape(displayScript), shutil.Escape(dir),
		shutil.Escape(displayOutputLog))
	testing.ContextLog(ctx, "Start display chart process: ", displayCmd)
	out, err = conn.CommandContext(ctx, "sh", "-c", displayCmd).Output()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to run display chart script")
	}
	pid = strings.TrimSpace(string(out))

	testing.ContextLog(ctx, "Poll for 'is ready' message for ensuring chart is ready")
	const chartReadyMsg = "Chart is ready."
	var fifo string
	fifoPathRegex := regexp.MustCompile(chartReadyMsg + ` Fifo:\s(\S+)`)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		output, err := conn.CommandContext(ctx, "grep", chartReadyMsg, displayOutputLog).Output()
		switch err.(type) {
		case nil:
			m := fifoPathRegex.FindSubmatch(output)
			if len(m) != 0 {
				testing.ContextLog(ctx, "Chart start in fifo mode")
				fifo = string(m[1])
			} else {
				return testing.PollBreak(errors.New("chart does not support fifo mode"))
			}
			return nil
		case *cryptossh.ExitError:
			// Grep failed to find ready message, wait for next poll.
			return err
		default:
			return testing.PollBreak(err)
		}
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return nil, nil, errors.Wrap(err, "failed to wait for chart ready")
	}
	testing.ContextLog(ctx, "Display chart complete")

	return &Chart{conn, dir, pid, fifo}, namePaths, nil
}

// Display displays the chart file specified by |namePath|.
func (c *Chart) Display(ctx context.Context, namePath NamePath) error {
	cmd := fmt.Sprintf(`echo '{"chart_name": %q}' > %s`, namePath, c.fifo)
	if err := c.conn.CommandContext(ctx, "bash", "-c", cmd).Run(); err != nil {
		return errors.Wrapf(err, "failed to change displayed chart to %v", namePath)
	}
	return nil
}

// SetDisplayLevel sets the display level ranged [0.0, 100.0].
func (c *Chart) SetDisplayLevel(ctx context.Context, lv float32) error {
	cmd := fmt.Sprintf(`echo '{"display_level": %.1f}' > %s`, lv, c.fifo)
	if err := c.conn.CommandContext(ctx, "bash", "-c", cmd).Run(); err != nil {
		return errors.Wrapf(err, "failed to change display level to %v", lv)
	}
	return nil
}

// Close closes the chart process and saves its logs to |outDir|.
func (c *Chart) Close(ctx context.Context, outDir string) error {
	if err := cleanup(ctx, c.conn, c.dir, c.pid, outDir); err != nil {
		return errors.Wrap(err, "failed to close chart")
	}
	return nil
}
