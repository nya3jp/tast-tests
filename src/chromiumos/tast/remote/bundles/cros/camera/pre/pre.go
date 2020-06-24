// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre provides test preconditions for displaying a chart on chart
// tablet paird with DUT in camerabox setup.
package pre

import (
	"context"
	"path"
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

// chartPre implements testing.Precondition.
type chartPre struct {
	// name is the name of chart.
	name string
	// conn is the SSH connection to the chart device.
	conn *ssh.Conn
	// dir is the directory saving all chart files on chart tablet.
	dir string
	// pid is the process id of running display chart script.
	pid string
	// prepared indicates whether the instance has been initialized by calling prepare().
	prepared bool
}

var dataCharts = map[string]*chartPre{
	"scene.pdf": &chartPre{name: "scene.pdf"},
}

// DataChart returns test precondition for displaying chart from s.DataPath(name) of test on chart tablet.
func DataChart(name string) *chartPre {
	return dataCharts[name]
}

func (p *chartPre) String() string         { return "chart_" + p.name }
func (p *chartPre) Timeout() time.Duration { return 2 * time.Minute }

// cleanupDisplayProcess cleans up running display chart process and saves process logs to outdir.
func (p *chartPre) cleanupDisplayProcess(ctx context.Context, outdir string) (retErr error) {
	if len(p.pid) == 0 {
		return errors.New("display script is not started")
	}
	defer func() {
		if err := linuxssh.GetFile(ctx, p.conn, displayOutputLog, path.Join(outdir, path.Base(displayOutputLog))); err != nil {
			if retErr != nil {
				testing.ContextLogf(ctx, "Failed to pull chart script logs from %v: %v", displayOutputLog, err)
			} else {
				retErr = errors.Wrapf(err, "failed pull chart script logs from %v", displayOutputLog)
			}
		}
		if err := p.conn.Command("rm", displayOutputLog).Run(ctx); err != nil {
			if retErr != nil {
				testing.ContextLogf(ctx, "Failed to clean up %v on chart tablet: %v", displayOutputLog, err)
			} else {
				retErr = errors.Wrapf(err, "failed to clean up %v on chart tablet", displayOutputLog)
			}
		}
	}()

	if err := p.conn.Command("kill", "-2", p.pid).Run(ctx); err != nil {
		return errors.Wrap(err, "failed to send terminate signal to display script")
	}
	// Here we assume the script terminate process should be very quick
	// and thus don't need to wait for its termination ends.

	return nil
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

func (p *chartPre) prepare(ctx context.Context, d *dut.DUT, altHostname, chartPath, outdir string) (retErr error) {
	// Connect to chart tablet.
	if len(altHostname) > 0 {
		conn, err := connectChart(ctx, d, altHostname)
		if err != nil {
			return errors.Wrapf(err, "failed to connect to chart with hostname %v", altHostname)
		}
		p.conn = conn
	} else {
		conn, err := d.DefaultCameraboxChart(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to connect to chart with default '-tablet' suffix hostname")
		}
		p.conn = conn
	}
	defer func() {
		if retErr != nil {
			if err := p.conn.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close SSH connection to chart tablet: ", err)
			}
		}
	}()

	// Create temp directory saving chart files.
	out, err := p.conn.Command("mktemp", "-d", "/tmp/chart_XXXXXX").Output(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create chart file directory on chart tablet")
	}
	p.dir = strings.TrimSpace(string(out))
	defer func() {
		if retErr != nil {
			if err := p.conn.Command("rm", "-rf", p.dir).Run(ctx); err != nil {
				testing.ContextLogf(ctx, "Failed remove chart directory %v: %v", p.dir, err)
			}
		}
	}()

	// Display chart on chart tablet.
	chartHostPath := path.Join(p.dir, p.name)
	if _, err := linuxssh.PutFiles(
		ctx, p.conn, map[string]string{chartPath: chartHostPath}, linuxssh.DereferenceSymlinks); err != nil {
		return errors.Wrapf(err, "failed to send chart file in path %v to chart tablet", chartPath)
	}
	displayCmd := shutil.EscapeSlice([]string{
		"(", "python2", displayScript, chartHostPath, ">", displayOutputLog, "2>&1", ")",
		"&", "echo", "-n", "$!"})
	testing.ContextLog(ctx, "Start display chart process: ", displayCmd)
	out, err = p.conn.Command("sh", "-c", displayCmd).Output(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to run display chart script")
	}
	p.pid = strings.TrimSpace(string(out))
	defer func() {
		if retErr != nil {
			if err := p.cleanupDisplayProcess(ctx, outdir); err != nil {
				testing.ContextLog(ctx, "Failed to clean up display chart process: ", err)
			}
		}
	}()

	testing.ContextLog(ctx, "Poll for 'is ready' message for ensuring chart is ready")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		err := p.conn.Command("grep", "-q", "Chart is ready.", displayOutputLog).Run(ctx)
		switch err.(type) {
		case nil, *cryptossh.ExitError:
			return err
		default:
			return testing.PollBreak(err)
		}
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for chart ready")
	}
	testing.ContextLog(ctx, "Display chart complete")
	p.prepared = true

	return nil
}

func (p *chartPre) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	if !p.prepared {
		var altHostname string
		if hostname, ok := s.Var("chart"); ok {
			altHostname = hostname
		}
		if err := p.prepare(ctx, s.DUT(), altHostname, s.DataPath(p.name), s.OutDir()); err != nil {
			s.Fatal("Failed to prepare chart tablet: ", err)
		}
	}
	return nil
}

func (p *chartPre) Close(ctx context.Context, s *testing.PreState) {
	if !p.prepared {
		return
	}

	if err := p.cleanupDisplayProcess(ctx, s.OutDir()); err != nil {
		s.Error("Failed to cleanup display chart process: ", err)
	}

	if err := p.conn.Command("rm", "-rf", p.dir).Run(ctx); err != nil {
		s.Errorf("Failed remove chart directory %v: %v", p.dir, err)
	}

	if err := p.conn.Close(ctx); err != nil {
		s.Error("Failed to close SSH connection to chart tablet: ", err)
	}
}
