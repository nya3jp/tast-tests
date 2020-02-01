// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromewpr starts a chrome instance and a WPR process.
package chromewpr

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

// Mode represents the mode to use in WPR, either record mode
// or replay mode.
type Mode int

// Replay vs. Record is the mode to use when running WPR
const (
	Replay Mode = iota
	Record
)

// Params contains the configurable parameters for New.
type Params struct {
	// Mode instructs New to start WPR in record mode
	// vs. replay mode.
	Mode Mode
	// WPRArchivePath is the path name of a WPR archive.
	WPRArchivePath string
	// UseLiveSites controls whether New should skip WPR
	// and have Chrome load pages directly from the internet.
	UseLiveSites bool
	// FakeLargeScreen instructs Chrome to use a large screen when
	// no displays are connected, which can happen, for instance, with
	// chromeboxes (otherwise Chrome will configure a default 1366x768
	// screen).
	FakeLargeScreen bool
	// UseARC indicates whether Chrome should be started with ARC enabled.
	UseARC bool
	Chrome *chrome.Chrome
}

// WPR is a struct containg a pointer to the Chrome Instance and
// the WPR process needed to run memory pressure.
type WPR struct {
	Chrome  *chrome.Chrome
	wprProc *testexec.Cmd
	HttpPort int
	HttpsPort int

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

// New starts a Chrome instance in preparation for testing. It returns a WPR
// pointer, which inludes a Chrome pointer, used for later communication with
// the browser, and a Cmd pointer for an already-started WPR process.
func New(ctx context.Context, p *Params) (*WPR, error) {
	var w WPR
	var opts []chrome.Option
	if p.UseARC {
		opts = append(opts, chrome.ARCEnabled())
	}
	if p.FakeLargeScreen {
		// The first flag makes Chrome create a larger window.  The
		// second flag is only effective if the device does not have a
		// display (chromeboxes).  Without it, Chrome uses a 1366x768
		// default.
		args := []string{"--ash-host-window-bounds=3840x2048", "--screen-config=3840x2048/i"}
		opts = append(opts, chrome.ExtraArgs(args...))
	}
	if p.UseLiveSites {
		testing.ContextLog(ctx, "Starting Chrome with live sites")
		if (p.Chrome != nil) {
			w.Chrome = p.Chrome
			w.wprProc = nil
			return &w, nil
		} else {
		cr, err := chrome.New(ctx, opts...)
		w.Chrome = cr
		w.wprProc = nil
		return &w, err
		}
	}
	var (
		tentativeCr  *chrome.Chrome
		tentativeWPR *testexec.Cmd
	)
	defer func() {
		if tentativeCr != nil {
			tentativeCr.Close(ctx)
		}
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
	httpPort := ports[0]
	httpsPort := ports[1]
	testing.ContextLogf(ctx, "Starting Chrome with WPR at ports %d and %d", httpPort, httpsPort)

	// Start the Web Page Replay process.  Normally this replays a supplied
	// WPR archive.  If recordPageSet is true, WPR records an archive
	// instead.
	//
	// The supplied WPR archive is stored in a private Google cloud storage
	// bucket and may not available in all setups.  In this case it must be
	// installed manually the first time the test is run on a DUT.  The GS
	// URL of the archive is contained in
	// data/memory_pressure_mixed_sites.wprgo.external.  The archive should
	// be copied to any location in the DUT (somewhere in /usr/local is
	// recommended) and the call to initBrowser should be updated to
	// reflect that location.
	var mode string
	switch p.Mode {
	case Replay:
		mode = "replay"
	case Record:
		mode = "record"
	default:
		return nil, errors.Errorf("unknown WPR mode %q", p.Mode)
	}
	testing.ContextLog(ctx, "Using WPR archive ", p.WPRArchivePath)
	tentativeWPR = testexec.CommandContext(ctx, "wpr", mode,
		fmt.Sprintf("--http_port=%d", httpPort),
		fmt.Sprintf("--https_port=%d", httpsPort),
		"--https_cert_file=/usr/local/share/wpr/wpr_cert.pem",
		"--https_key_file=/usr/local/share/wpr/wpr_key.pem",
		"--inject_scripts=/usr/local/share/wpr/deterministic.js",
		p.WPRArchivePath)

	if err := tentativeWPR.Start(); err != nil {
		tentativeWPR.DumpLog(ctx)
		return nil, errors.Wrap(err, "cannot start WPR")
	}

	// Restart chrome for use with WPR.  Chrome can start before WPR is
	// ready because it will not need it until we start opening tabs.
	// resolverRules := fmt.Sprintf("MAP *:80 127.0.0.1:%d,MAP *:443 127.0.0.1:%d,EXCLUDE localhost",
	// 	httpPort, httpsPort)
	// resolverRulesFlag := fmt.Sprintf("--host-resolver-rules=%q", resolverRules)
	// spkiList := "PhrPvGIaAMmd29hj8BCZOq096yj7uMpRNHpn5PDxI6I="
	// spkiListFlag := fmt.Sprintf("--ignore-certificate-errors-spki-list=%s", spkiList)
	// args := []string{resolverRulesFlag, spkiListFlag}
	// opts = append(opts, chrome.ExtraArgs(args...))
	// tentativeCr, err = chrome.New(ctx, opts...)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "cannot start Chrome")
	// }

	// Wait for WPR to initialize.
	httpSocketName := fmt.Sprintf("localhost:%d", httpPort)
	httpsSocketName := fmt.Sprintf("localhost:%d", httpsPort)
	if err := waitForServerSocket(ctx, httpSocketName, tentativeWPR); err != nil {
		return nil, errors.Wrapf(err, "cannot connect to WPR at %s", httpSocketName)
	}

	testing.ContextLog(ctx, "WPR HTTPS socket is up at ", httpsSocketName)
	w.Chrome = tentativeCr
	tentativeCr = nil
	w.wprProc = tentativeWPR
	tentativeWPR = nil
	w.HttpPort = httpPort
	w.HttpsPort = httpsPort
	return &w, nil

}

// Close closes Chrome, and sends SIGINT to the WPR process.
func (w *WPR) Close(ctx context.Context) error {
	var firstErr error
	if err := w.Chrome.Close(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	if w.wprProc != nil {
		// send SIGINT to exit properly in recording mode.
		if err := w.wprProc.Signal(syscall.SIGINT); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := w.wprProc.Wait(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
