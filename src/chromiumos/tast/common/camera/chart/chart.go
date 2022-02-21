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
	// display configuration into it. Can be empty if the chart tablet
	// don't support fifo mode.
	fifo string
}

var errFifoModeNotSupported = errors.New("fifo mode not supported on tablet")

// IsErrFifoModeNotSupported returns whether the error comes from chart don't support fifo mode.
func IsErrFifoModeNotSupported(err error) bool {
	if err == nil {
		return false
	}
	if err == errFifoModeNotSupported {
		return true
	}
	if wrappedErr, ok := err.(*errors.E); ok {
		return IsErrFifoModeNotSupported(wrappedErr.Unwrap())
	}
	return false
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

// NewWithDisplayLevel displays |chartPath| chart on either |altHostname| or |d|'s
// corresponding chart tablet and returns a new |Chart| instance.
// It uses |displayLevel| to set the brightness, the range is in [0.0, 100.0].
func NewWithDisplayLevel(ctx context.Context, d *dut.DUT, altHostname, chartPath, outDir string, displayLevel float32) (_ *Chart, retErr error) {
	var conn *ssh.Conn

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

	return SetUp(ctx, conn, chartPath, outDir, displayLevel)
}

// New displays |chartPath| chart on either |altHostname| or |d|'s
// corresponding chart tablet and returns a new |Chart| instance.
func New(ctx context.Context, d *dut.DUT, altHostname, chartPath, outDir string) (_ *Chart, retErr error) {
	return NewWithDisplayLevel(ctx, d, altHostname, chartPath, outDir, DisplayDefaultLevel)
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
func SetUp(ctx context.Context, conn *ssh.Conn, chartPath, outDir string, displayLevel float32) (_ *Chart, retErr error) {
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
		return nil, errors.Wrap(err, "failed to create chart file directory on chart tablet")
	}
	dir = strings.TrimSpace(string(out))

	// Display chart on chart tablet.
	if err := copyChart(ctx, conn, chartPath, dir); err != nil {
		return nil, err
	}
	var displayLevelOpt string
	if displayLevel >= 0.0 {
		displayLevelOpt = fmt.Sprintf("--display_level=%f", displayLevel)
	}
	chartHostPath := filepath.Join(dir, filepath.Base(chartPath))
	displayCmd := fmt.Sprintf(
		"(python2 %s %s %s > %s 2>&1) & echo -n $!",
		shutil.Escape(displayScript), shutil.Escape(chartHostPath),
		displayLevelOpt, shutil.Escape(displayOutputLog))
	testing.ContextLog(ctx, "Start display chart process: ", displayCmd)
	out, err = conn.CommandContext(ctx, "sh", "-c", displayCmd).Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run display chart script")
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
				testing.ContextLog(ctx, "Chart start in non-fifo mode")
			}
			return nil
		case *cryptossh.ExitError:
			// Grep failed to find ready message, wait for next poll.
			return err
		default:
			return testing.PollBreak(err)
		}
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for chart ready")
	}
	testing.ContextLog(ctx, "Display chart complete")

	return &Chart{conn, dir, pid, fifo}, nil
}

// Display change the displayed chart.
func (c *Chart) Display(ctx context.Context, chartPath string) error {
	if len(c.fifo) == 0 {
		return errFifoModeNotSupported
	}

	if err := copyChart(ctx, c.conn, chartPath, c.dir); err != nil {
		return err
	}

	cmd := fmt.Sprintf(`echo '{"chart_name": %q}' > %s`, filepath.Base(chartPath), c.fifo)
	if err := c.conn.CommandContext(ctx, "bash", "-c", cmd).Run(); err != nil {
		return errors.Wrapf(err, "failed to change displayed chart to %v", chartPath)
	}
	return nil
}

// SetDisplayLevel sets the display level ranged [0.0, 100.0].
func (c *Chart) SetDisplayLevel(ctx context.Context, lv float32) error {
	if len(c.fifo) == 0 {
		return errFifoModeNotSupported
	}

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

// Helper is a helper class to control display level and change chart easily.
type Helper struct {
	chart        *Chart
	ctx          context.Context
	d            *dut.DUT
	altHostname  string
	chartPath    string
	outDir       string
	displayLevel float32
}

// NewHelper creates |Helper|.
func NewHelper(ctx context.Context, d *dut.DUT, altHostname, chartPath, outDir string, displayLevel float32) (*Helper, error) {
	chart, err := NewWithDisplayLevel(ctx, d, altHostname, chartPath, outDir, displayLevel)
	if err != nil {
		return nil, err
	}
	return &Helper{chart, ctx, d, altHostname, chartPath, outDir, displayLevel}, nil
}

// SetDisplayLevel sets the display level.
func (h *Helper) SetDisplayLevel(displayLevel float32) error {
	if h.chart != nil {
		// Try to reuse the existing chart service to set new display level if fifo mode is supported.
		err := h.chart.SetDisplayLevel(h.ctx, displayLevel)
		if err == nil {
			return nil
		}
		if !IsErrFifoModeNotSupported(err) {
			return err
		}
		// Fifo mode not support, fallback to legacy mode restarting chart service every time.
		if err := h.chart.Close(h.ctx, h.outDir); err != nil {
			return err
		}
		h.chart = nil
	}
	chart, err := NewWithDisplayLevel(h.ctx, h.d, h.altHostname, h.chartPath, h.outDir, displayLevel)
	if err != nil {
		return err
	}
	h.chart = chart
	h.displayLevel = displayLevel
	return nil
}

// Close closes the helper.
func (h *Helper) Close() error {
	if h.chart != nil {
		return h.chart.Close(h.ctx, h.outDir)
	}
	return nil
}
