// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type HostAPServer struct {
	host       *dut.DUT
	conf       *HostAPConfig
	confPath   string
	stdoutPath string
	stderrPath string
	ctrlPath   string
	iface      string
	pid        int
}

func NewHostAPServer(r *Router, c *HostAPConfig) *HostAPServer {
	return &HostAPServer{
		host: r.host,
		conf: c,
	}
}

func (ap *HostAPServer) Start(ctx context.Context, iface string) error {
	ap.iface = iface
	for _, p := range []struct {
		field  *string
		format string
	}{
		{&ap.confPath, "/tmp/hostapd-%s.conf"},
		{&ap.stdoutPath, "/tmp/hostapd-%s.ctrl"},
		{&ap.stderrPath, "/tmp/hostapd-%s.stdout"},
		{&ap.ctrlPath, "/tmp/hostapd-%s.stderr"},
	} {
		*p.field = fmt.Sprintf(p.format, iface)
	}

	cmd := fmt.Sprintf("cat > %s <<EOF\n%sEOF\n", ap.confPath, ap.conf.Format(ap.iface, ap.ctrlPath))
	cmd = cmd + fmt.Sprintf("hostapd -dd -t %s >%s 2>%s & echo $!", ap.confPath, ap.stdoutPath, ap.stderrPath)
	out, err := ap.host.Run(ctx, cmd)
	if err != nil {
		testing.ContextLog(ctx, string(out))
		return errors.Wrap(err, "hostapd failure")
	}
	ap.pid, err = strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return errors.Wrap(err, "parsing hostapd PID")
	}

	var abort error
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := ap.host.Run(ctx, fmt.Sprintf("cat %s", ap.stdoutPath))
		if err != nil {
			return err
		}
		s := string(out)

		if strings.Contains(s, "Setup of interface done") {
			return nil
		}
		if strings.Contains(s, "Interface initialization failed") {
			// Don't keep polling. We failed.
			abort = errors.New("hostapd failed to initialize AP interface")
			return nil
		}
		if !ap.IsRunning(ctx) {
			// Don't keep polling. We failed.
			abort = errors.New("hostapd process terminated")
			return nil
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	if abort != nil {
		return abort
	}

	return nil
}

func (ap *HostAPServer) Stop(ctx context.Context) error {
	if !ap.IsRunning(ctx) {
		return nil
	}
	if _, err := ap.host.Run(ctx, fmt.Sprintf("kill %d\n", ap.pid)); err != nil {
		return errors.Wrapf(err, "killing PID %d failed", ap.pid)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if !ap.IsRunning(ctx) {
			return nil
		}
		return errors.New(fmt.Sprintf("hostapd process %d still running", ap.pid))
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrapf(err, "hostapd PID %d didn't stop", ap.pid)
	}

	ap.pid = 0
	return nil
}

func (ap *HostAPServer) IsRunning(ctx context.Context) bool {
	if ap.pid == 0 {
		return false
	}
	if _, err := ap.host.Run(ctx, fmt.Sprintf("kill -0 %d", ap.pid)); err != nil {
		return false
	}
	return true
}
