// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpr manages a Web Page Replay (aka WPR) process and provides
// chrome.Options to configure Chrome to send all web traffic through the WPR
// process.
package wpr

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Mode represents the mode to use in WPR, either record mode or replay mode.
type Mode int

// Replay vs. Record is the mode to use when running WPR.
const (
	Replay Mode = iota
	Record
)

func (m Mode) String() string {
	switch m {
	case Replay:
		return "replay"
	case Record:
		return "record"
	default:
		return ""
	}
}

// WPR holds information about WPR process and chrome.Options to configure
// Chrome to send traffic through the WPR process.
type WPR struct {
	HTTPPort      int
	HTTPSPort     int
	ChromeOptions []chrome.Option
	proc          *testexec.Cmd
}

// availableTCPPorts returns a list of TCP ports on localhost that are not in
// use.  Returns an error if one or more ports cannot be allocated.  Note that
// the ports are not reserved, but chances that they remain available for at
// least a short time after this call are very high.
func availableTCPPorts(count int) ([]int, error) {
	var ls []net.Listener
	defer func() {
		for _, l := range ls {
			l.Close()
		}
	}()
	var ports []int
	for i := 0; i < count; i++ {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	return ports, nil
}

// waitForServerSocket tries to connect to a TCP address, which is a string in
// the form "host:port", e.g. "localhost:8080", served by server, which is an
// already-started server process. If connecting to the address fails,
// server.DumpLog is called to log more information.
func waitForServerSocket(ctx context.Context, addr string, server *testexec.Cmd) error {
	err := testing.Poll(ctx, func(ctx context.Context) error {
		d := &net.Dialer{Timeout: time.Second}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}, &testing.PollOptions{
		Interval: time.Second,
		Timeout:  60 * time.Second,
	})
	return err
}

// New starts a WPR process and prepares chrome.Options to configure Chrome
// to send all web traffic through the WPR process.
func New(ctx context.Context, mode Mode, archive string) (*WPR, error) {
	ports, err := availableTCPPorts(2)
	if err != nil {
		return nil, errors.Wrap(err, "cannot allocate WPR ports")
	}
	httpPort := ports[0]
	httpsPort := ports[1]
	testing.ContextLogf(ctx, "Starting WPR with ports %d and %d",
		httpPort, httpsPort)

	// Start the Web Page Replay process.  Normally this replays a supplied
	// WPR archive.  If p.mode is Record, WPR records an archive instead.
	m := mode.String()
	if m == "" {
		return nil, errors.Errorf("unknown WPR mode %q", mode)
	}
	testing.ContextLog(ctx, "Using WPR archive ", archive)
	proc := testexec.CommandContext(ctx, "wpr", m,
		fmt.Sprintf("--http_port=%d", httpPort),
		fmt.Sprintf("--https_port=%d", httpsPort),
		"--https_cert_file=/usr/local/share/wpr/wpr_cert.pem",
		"--https_key_file=/usr/local/share/wpr/wpr_key.pem",
		"--inject_scripts=/usr/local/share/wpr/deterministic.js",
		archive)

	if err := proc.Start(); err != nil {
		return nil, errors.Wrap(err, "cannot start WPR")
	}
	defer func() {
		if proc != nil {
			if err := proc.Kill(); err != nil {
				testing.ContextLog(ctx, "Cannot kill WPR: ", err)
			}
			if err := proc.Wait(); err != nil {
				testing.ContextLog(ctx, "Failed to release WPR resources: ", err)
			}
		}
	}()

	// Wait for WPR http socket.
	httpSocketName := fmt.Sprintf("localhost:%d", httpPort)
	if err := waitForServerSocket(ctx, httpSocketName, proc); err != nil {
		return nil, errors.Wrapf(err, "cannot connect to WPR at %s", httpSocketName)
	}
	testing.ContextLog(ctx, "WPR HTTP socket is up at ", httpSocketName)

	// Build chrome.Options to configure Chrome to send traffic through WPR.
	resolverRules := fmt.Sprintf(
		"MAP *:80 127.0.0.1:%d,MAP *:443 127.0.0.1:%d,EXCLUDE localhost",
		httpPort, httpsPort)
	resolverRulesFlag := fmt.Sprintf("--host-resolver-rules=%q", resolverRules)
	const spkiList = "PhrPvGIaAMmd29hj8BCZOq096yj7uMpRNHpn5PDxI6I="
	spkiListFlag := fmt.Sprintf("--ignore-certificate-errors-spki-list=%s", spkiList)
	args := []string{resolverRulesFlag, spkiListFlag}
	opts := []chrome.Option{chrome.ExtraArgs(args...)}

	wpr := &WPR{
		HTTPPort:      httpPort,
		HTTPSPort:     httpsPort,
		ChromeOptions: opts,
		proc:          proc,
	}
	proc = nil // Skip the deferred kill on success.
	return wpr, nil
}

// Close sends SIGINT to the WPR process.
func (w *WPR) Close(ctx context.Context) error {
	var firstErr error
	if w.proc != nil {
		// Send SIGINT to exit properly in recording mode.
		if err := w.proc.Signal(syscall.SIGINT); err != nil {
			firstErr = err
		}

		ctx, cancel := context.WithCancel(ctx)
		defer cancel() // Make sure goroutine is unblocked on exit.

		proc := w.proc
		go func() {
			<-ctx.Done()
			proc.Signal(syscall.SIGKILL)
		}()

		if err := w.proc.Wait(); err != nil && firstErr == nil {
			// Check whether wpr was terminated with SIGINT from above.
			ws, ok := testexec.GetWaitStatus(err)
			if !ok {
				firstErr = errors.Wrap(err, "failed to get wait status")
			} else if !ws.Signaled() || ws.Signal() != syscall.SIGINT {
				firstErr = err
			}
		}
		w.proc = nil
	}
	return firstErr
}
