// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cdputil

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/rpcc"
	"github.com/mafredri/cdp/session"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	// DebuggingPortPath is a file where Chrome writes debugging port.
	DebuggingPortPath = "/home/chronos/DevToolsActivePort"

	// writeBufferSize is a larger default buffer size (1 MB) for websocket connection.
	writeBufferSize = 1048576
)

// Session maintains the connection to talk to the browser in Chrome DevTools Protocol
// over WebSocket.
type Session struct {
	addr   string     // DevTools address, including port.
	wsConn *rpcc.Conn // DevTools WebSocket connection to the browser.

	// TODO(hidehiko): Make these private for better encapsulation.
	Client  *cdp.Client      // DevTools client using wsConn.
	Manager *session.Manager // Manages connections to multiple targets over wsConn.
}

// NewSession establishes a Chrome DevTools Protocol WebSocket connection to the browser.
func NewSession(ctx context.Context) (sess *Session, retErr error) {
	port, err := waitForDebuggingPort(ctx)
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// The /json/version HTTP endpoint provides the browser's WebSocket URL.
	// See https://chromedevtools.github.io/devtools-protocol/ for details.
	// To avoid mixing HTTP and WS requests, we use only WS after this.
	version, err := devtool.New("http://" + addr).Version(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query browser's HTTP endpoint")
	}

	testing.ContextLog(ctx, "Connecting to browser at ", version.WebSocketDebuggerURL)
	co, err := rpcc.DialContext(ctx, version.WebSocketDebuggerURL, rpcc.WithWriteBufferSize(writeBufferSize))
	if err != nil {
		return nil, errors.Wrap(err, "failed to establish WebSocket connection to browser")
	}
	defer func() {
		if retErr != nil {
			co.Close()
		}
	}()

	cl := cdp.NewClient(co)

	// This lets us manage multiple targets using a single WebSocket connection.
	m, err := session.NewManager(cl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create session.Manager")
	}

	return &Session{
		addr:    addr,
		wsConn:  co,
		Client:  cl,
		Manager: m,
	}, nil
}

// waitForDebuggingPort waits for Chrome's debugging port to become available.
// Returns the port number.
func waitForDebuggingPort(ctx context.Context) (int, error) {
	testing.ContextLog(ctx, "Waiting for Chrome to write its debugging port to ", DebuggingPortPath)
	ctx, st := timing.Start(ctx, "wait_for_debugging_port")
	defer st.End()

	var port int
	if err := testing.Poll(ctx, func(context.Context) error {
		var err error
		port, err = readDebuggingPort(DebuggingPortPath)
		return err
	}, &testing.PollOptions{Interval: 10 * time.Millisecond}); err != nil {
		return -1, errors.Wrap(err, "failed to read Chrome debugging port")
	}

	return port, nil
}

// readDebuggingPort returns the port number from the first line of p, a file
// written by Chrome when --remote-debugging-port=0 is passed.
func readDebuggingPort(p string) (int, error) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return -1, err
	}
	lines := strings.SplitN(string(b), "\n", 2) // We only need the first line of the file.
	return strconv.Atoi(lines[0])
}

// Close shuts down the connection to the browser.
func (s *Session) Close() error {
	var errs []error
	if err := s.Manager.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := s.wsConn.Close(); err != nil {
		errs = append(errs, err)
	}
	if errs != nil {
		return errors.Errorf("failed to close Session: %v", errs)
	}

	return nil
}

// DebugAddrPort returns the addr:port at which Chrome is listening for DevTools connections,
// e.g. "127.0.0.1:38725".
func (s *Session) DebugAddrPort() string {
	return s.addr
}
