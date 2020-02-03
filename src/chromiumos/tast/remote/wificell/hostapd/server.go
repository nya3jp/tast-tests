// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/remote/wificell/fileutils"
	"chromiumos/tast/testing"
)

const (
	hostapdCmd = "hostapd"
)

// Killall kills all running hostapd on host, useful for environment setup/cleanup.
func Killall(ctx context.Context, host *host.SSH) error {
	return host.Command("killall", hostapdCmd).Run(ctx)
}

// Server controls a hostapd on router.
type Server struct {
	host    *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	name    string
	iface   string
	workDir string
	conf    *Config
	cmd     *host.Cmd
}

// NewServer creates a new Server object and runs hostapd on iface of the given host with settings
// specified in config. workDir is the dir on host for the server to put temporary files.
// name is the identifier used for log filenames in OutDir.
func NewServer(host *host.SSH, name, iface, workDir string, config *Config) *Server {
	return &Server{
		host:    host,
		name:    name,
		iface:   iface,
		workDir: workDir,
		conf:    config,
	}
}

// filename for this instance to store different type of information.
// suffix can be the type of stored information. e.g. conf, stdout, stderr ...
func (ap *Server) filename(suffix string) string {
	return fmt.Sprintf("hostapd-%s-%s.%s", ap.name, ap.iface, suffix)
}

// confPath is the location on host of hostapd.conf for this instance.
func (ap *Server) confPath() string {
	return path.Join(ap.workDir, ap.filename("conf"))
}

// ctrlPath is the location on host of control socket of this hostapd.
func (ap *Server) ctrlPath() string {
	return path.Join(ap.workDir, ap.filename("ctrl"))
}

// stdoutFile is the filename under OutDir to store stdout of this hostapd.
func (ap *Server) stdoutFile() string {
	return ap.filename("stdout")
}

// stderrFile is the filename under OutDir to store stderr of this hostapd.
func (ap *Server) stderrFile() string {
	return ap.filename("stderr")
}

// spawnStdoutParser in background to make sure hostapd is properly started and to detect
// events. This function returns a channel to indicate if hostapd gets ready without error.
func (ap *Server) spawnStdoutParser(ctx context.Context, stdout io.Reader) chan error {
	done := make(chan error, 1)
	go func() {
		var msg []byte
		buf := make([]byte, 1024)
		// Create pipe between event/ready detecter and stdout logger.
		pr, pw := io.Pipe()
		defer pw.Close()

		// Collect stdout to OutDir.
		if err := fileutils.ReadToOutDir(ctx, ap.stdoutFile(), pr); err != nil {
			done <- errors.Wrap(err, "failed to spawn reader for stdout")
			return
		}
		// Wait for service ready.
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				// If the program exits unexpectedly, it will also go here with
				// err = io.EOF.
				done <- errors.Wrap(err, "failed to read stdout of hostapd")
				return
			}
			// Write to pipe for logging to OutDir.
			if _, err := pw.Write(buf[:n]); err != nil {
				done <- errors.Wrap(err, "failed to pipe to log writer")
				return
			}
			// Check if we're done.
			msg = append(msg, buf[:n]...)
			s := string(msg)
			if strings.Contains(s, "Setup of interface done") {
				break
			}
			if strings.Contains(s, "Interface initialization failed") {
				// Don't keep waiting. We failed.
				done <- errors.New("hostapd failed to initialize AP interface")
				return
			}
		}
		// Service ready.
		close(done)
		// TODO(crbug.com/1034875): detect events from log (e.g. deauth, disconnect...)
		// This should be checked by SimpleConnect test.
		io.Copy(pw, stdout)
	}()
	return done
}

// Start hostapd daemon and wait until it is ready.
func (ap *Server) Start(ctx context.Context) (err error) {
	// Cleanup on error.
	defer func() {
		if err != nil {
			ap.Stop(ctx)
		}
	}()

	conf, err := ap.conf.Format(ap.iface, ap.ctrlPath())
	if err != nil {
		return err
	}
	err = fileutils.WriteToHost(ctx, ap.host, ap.confPath(), []byte(conf))
	if err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLogf(ctx, "Starting hostapd %s on interface %s", ap.name, ap.iface)
	cmd := ap.host.Command(hostapdCmd, "-dd", "-t", ap.confPath())
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StdoutPipe of hostapd")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StderrPipe of hostapd")
	}
	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	ap.cmd = cmd

	// Collect stderr.
	if err := fileutils.ReadToOutDir(ctx, ap.stderrFile(), stderr); err != nil {
		return errors.Wrap(err, "failed to spawn reader for stderr")
	}

	// Wait for hostapd to get ready.
	ready := ap.spawnStdoutParser(ctx, stdout)
	select {
	case err = <-ready:
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "hostapd started")
	return nil
}

// Stop hostapd and clean up related resources.
func (ap *Server) Stop(ctx context.Context) error {
	testing.ContextLog(ctx, "Stopping hostapd")
	var err error
	if ap.cmd == nil {
		err = errors.New("server not started")
	} else {
		ap.cmd.Abort()
		// TODO(crbug.com/1030635): Abort might not work, use pkill to ensure the daemon is killed.
		ap.host.Command("pkill", "-f", fmt.Sprintf("^%s.*%s", hostapdCmd, ap.confPath()))

		// Skip the error in Wait as the process is aborted and always has error in wait.
		ap.cmd.Wait(ctx)
		ap.cmd = nil
	}
	if err2 := ap.host.Command("rm", ap.confPath()).Run(ctx); err2 != nil {
		err = errors.Wrapf(err, "failed to remove config with err=%s", err2.Error())
	}
	return err
}

// Interface returns the interface used by the hostapd.
func (ap *Server) Interface() string {
	return ap.iface
}

// Config returns the config used by the hostapd.
func (ap *Server) Config() Config {
	return *ap.conf
}
