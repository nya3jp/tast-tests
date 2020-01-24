// Copyright 2019 The Chromium OS Authors. All rights reserved.
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

// Params contains the configurable parameters for New.
type Params struct {
	// Mode instructs New to start WPR in record mode vs. replay mode.
	Mode Mode
	// WPRArchivePath is the path name of a WPR archive.
	WPRArchivePath string
}

// WPR holds information about wpr process and chrome.Options to configure
// Chrome to send traffic through the wpr process.
type WPR struct {
	HTTPPort      int
	HTTPSPort     int
	ChromeOptions []chrome.Option
	wprProc       *testexec.Cmd
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

// waitForServerSocket tries to connect to a TCP socket, which is a string in
// the form "host:port", e.g. "localhost:8080", served by server, which is an
// already-started server process. If connecting to the socket fails,
// server.DumpLog is called to log more information.
func waitForServerSocket(ctx context.Context, socket string, server *testexec.Cmd) error {
	err := testing.Poll(ctx, func(ctx context.Context) error {
		conn, err := net.Dial("tcp", socket)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}, &testing.PollOptions{
		Interval: 1 * time.Second,
		Timeout:  60 * time.Second,
	})
	if err != nil {
		// Try to collect the server log to understand why we could not connect.
		if dumpErr := server.DumpLog(ctx); dumpErr != nil {
			// Log error but do not return it since the earlier
			// error is more informative.
			testing.ContextLog(ctx, "Could not dump server log: ", dumpErr)
		}
	}
	return err
}

// New starts a WPR process and prepares chrome.Options to configure chrome
// to send all web traffic through the WPR process.
func New(ctx context.Context, p *Params) (*WPR, error) {
	var w WPR
	var tentativeWPR *testexec.Cmd
	defer func() {
		if tentativeWPR != nil {
			if err := tentativeWPR.Kill(); err != nil {
				testing.ContextLog(ctx, "Cannot kill WPR: ", err)
			}
			if err := tentativeWPR.Wait(); err != nil {
				testing.ContextLog(ctx, "Failed to release WPR resources: ", err)
			}
		}
	}()

	ports, err := availableTCPPorts(2)
	if err != nil {
		return nil, errors.Wrap(err, "cannot allocate WPR ports")
	}
	w.HTTPPort = ports[0]
	w.HTTPSPort = ports[1]
	testing.ContextLogf(ctx, "Starting WPR with ports %d and %d",
		w.HTTPPort, w.HTTPSPort)

	// Start the Web Page Replay process.  Normally this replays a supplied
	// WPR archive.  If p.mode is Record, WPR records an archive instead.
	mode := p.Mode.String()
	if mode == "" {
		return nil, errors.Errorf("unknown WPR mode %q", p.Mode)
	}
	testing.ContextLog(ctx, "Using WPR archive ", p.WPRArchivePath)
	tentativeWPR = testexec.CommandContext(ctx, "wpr", mode,
		fmt.Sprintf("--http_port=%d", w.HTTPPort),
		fmt.Sprintf("--https_port=%d", w.HTTPSPort),
		"--https_cert_file=/usr/local/share/wpr/wpr_cert.pem",
		"--https_key_file=/usr/local/share/wpr/wpr_key.pem",
		"--inject_scripts=/usr/local/share/wpr/deterministic.js",
		p.WPRArchivePath)

	if err := tentativeWPR.Start(); err != nil {
		tentativeWPR.DumpLog(ctx)
		return nil, errors.Wrap(err, "cannot start WPR")
	}

	// Build chrome.Options to configure Chrome to send traffic through WPR.
	resolverRules := fmt.Sprintf(
		"MAP *:80 127.0.0.1:%d,MAP *:443 127.0.0.1:%d,EXCLUDE localhost",
		w.HTTPPort, w.HTTPSPort)
	resolverRulesFlag := fmt.Sprintf("--host-resolver-rules=%q", resolverRules)
	spkiList := "PhrPvGIaAMmd29hj8BCZOq096yj7uMpRNHpn5PDxI6I="
	spkiListFlag := fmt.Sprintf("--ignore-certificate-errors-spki-list=%s", spkiList)
	args := []string{resolverRulesFlag, spkiListFlag}
	w.ChromeOptions = append(w.ChromeOptions, chrome.ExtraArgs(args...))

	w.wprProc = tentativeWPR
	tentativeWPR = nil
	return &w, nil
}

// Wait waits for the WPR process http port to come up.
func (w *WPR) Wait(ctx context.Context) error {
	httpSocketName := fmt.Sprintf("localhost:%d", w.HTTPPort)
	if err := waitForServerSocket(ctx, httpSocketName, w.wprProc); err != nil {
		return errors.Wrapf(err, "cannot connect to WPR at %s", httpSocketName)
	}

	testing.ContextLog(ctx, "WPR HTTP socket is up at ", httpSocketName)
	return nil
}

// Close sends SIGINT to the WPR process.
func (w *WPR) Close() error {
	var firstErr error
	if w.wprProc != nil {
		// send SIGINT to exit properly in recording mode.
		if err := w.wprProc.Signal(syscall.SIGINT); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := w.wprProc.Wait(); err != nil && firstErr == nil {
			// Check whether wpr was terminated with SIGINT from above.
			ws, ok := testexec.GetWaitStatus(err)
			if !ok {
				firstErr = errors.Wrap(err, "failed to get wait status")
			} else if !ws.Signaled() || ws.Signal() != syscall.SIGINT {
				firstErr = err
			}
		}
	}
	return firstErr
}
