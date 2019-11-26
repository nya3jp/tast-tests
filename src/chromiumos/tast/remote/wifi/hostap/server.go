// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostap

import (
	"context"
	"fmt"
	"path"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/remote/wifi/utils"
	"chromiumos/tast/testing"
)

// Ported from Brian's draft crrev.com/c/1733740.

const (
	hostapdCmd = "hostapd"
)

// Server controls a hostapd on router.
type Server struct {
	host    *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	conf    *Config
	iface   string
	workDir string
	cmd     *host.Cmd
}

// NewServer creates a new Server object and runs hostapd on iface of the given host with settings
// specified in config. workDir is the dir on host for the server to put temporary files.
func NewServer(ctx context.Context, host *host.SSH, iface, workDir string, config *Config) (*Server, error) {
	server := &Server{
		host:    host,
		conf:    config,
		iface:   iface,
		workDir: workDir,
	}
	if err := server.start(ctx); err != nil {
		return nil, err
	}
	return server, nil
}

func (ap *Server) confPath() string {
	return path.Join(ap.workDir, fmt.Sprintf("hostapd-%s.conf", ap.iface))
}

func (ap *Server) ctrlPath() string {
	return path.Join(ap.workDir, fmt.Sprintf("hostapd-%s.ctrl", ap.iface))
}

func (ap *Server) stdoutFile() string {
	return fmt.Sprintf("hostapd-%s.stdout", ap.iface)
}

func (ap *Server) stderrFile() string {
	return fmt.Sprintf("hostapd-%s.stderr", ap.iface)
}

func (ap *Server) start(ctx context.Context) (err error) {
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
	err = utils.WriteToHost(ctx, ap.host, ap.confPath(), []byte(conf))
	if err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLog(ctx, "Starting hostapd")
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
	if err := utils.ReadToOutDir(ctx, ap.stderrFile(), stderr, nil); err != nil {
		return errors.Wrap(err, "failed to spawn reader for stderr")
	}

	// Wait for hostapd to get ready.
	done := make(chan error, 1)
	go func() {
		var msg []byte
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				// If the program exits unexpectedly, it will also go here with
				// err = io.EOF.
				done <- errors.Wrap(err, "failed to read stdout of hostapd")
				return
			}
			msg = append(msg, buf[:n]...)
			s := string(msg)
			if strings.Contains(s, "Setup of interface done") {
				break
			}
			if strings.Contains(s, "Interface initialization failed") {
				// Don't keep polling. We failed.
				done <- errors.New("hostapd failed to initialize AP interface")
				return
			}
		}
		if err := utils.ReadToOutDir(ctx, ap.stdoutFile(), stdout, msg); err != nil {
			done <- errors.Wrap(err, "failed to spawn reader for stdout")
			return
		}
		// Service ready.
		close(done)
	}()

	select {
	case err = <-done:
	case <-ctx.Done():
		err = ctx.Err()
	}

	if err != nil {
		return err
	}
	testing.ContextLog(ctx, "hostapd started")
	return nil
}

// Stop hostapd and cleanup related resources.
func (ap *Server) Stop(ctx context.Context) error {
	testing.ContextLog(ctx, "Stoping hostapd")
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
