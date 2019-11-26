// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/testing"
)

// Port from Brian's PoC crrev.com/c/1733740

// HostAPServer is the object to control the hostapd on router.
type HostAPServer struct {
	host     *host.SSH // TODO(crbug.com/1019537): use a more suitable ssh object.
	conf     *HostAPConfig
	confPath string
	ctrlPath string
	iface    string
	cmd      *host.Cmd
}

// NewHostAPServer creates a new HostAPServer and runs hostapd on the router.
func NewHostAPServer(ctx context.Context, r *Router, iface string, c *HostAPConfig) (*HostAPServer, error) {
	server := &HostAPServer{
		host:  r.host,
		conf:  c,
		iface: iface,
	}
	if err := server.start(ctx); err != nil {
		return nil, err
	}
	return server, nil
}

func (ap *HostAPServer) start(ctx context.Context) error {
	ap.confPath = fmt.Sprintf("/tmp/hostapd-%s.conf", ap.iface)
	ap.ctrlPath = fmt.Sprintf("/var/run/hostapd-%s", ap.iface)

	conf, err := ap.conf.Format(ap.iface, ap.ctrlPath)
	if err != nil {
		return err
	}
	err = writeToHost(ctx, ap.host, ap.confPath, []byte(conf))
	if err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	testing.ContextLog(ctx, "Starting hostapd")
	cmd := ap.host.Command("hostapd", "-dd", "-t", ap.confPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to obtain StdoutPipe of hostapd")
	}
	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	ap.cmd = cmd

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
			}
		}
		// Service ready, free resources.
		close(done)
		msg = nil
		// Drain the remaining stdout till EOF.
		// We cannot just close the pipe or else the writer will be blocked and write call
		// in remote program gets blocked in the end.
		io.Copy(ioutil.Discard, stdout)
	}()

	select {
	case err = <-done:
	case <-ctx.Done():
		err = ctx.Err()
	}

	if err != nil {
		ap.Stop(ctx)
		return err
	}
	testing.ContextLog(ctx, "hostapd started")
	return nil
}

// Stop the HostAPServer.
func (ap *HostAPServer) Stop(ctx context.Context) error {
	if ap.cmd == nil {
		return errors.New("server not started")
	}

	ap.cmd.Abort()
	// Skip the error in Wait as the process is aborted and always has error in wait.
	ap.cmd.Wait(ctx)
	ap.cmd = nil
	if err := ap.host.Command("rm", ap.confPath).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to remove config with err=%s", err.Error())
	}
	return nil
}
