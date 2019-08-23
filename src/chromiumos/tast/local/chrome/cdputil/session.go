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
	"github.com/mafredri/cdp/protocol/target"
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
	addr   string      // DevTools address, including port (e.g. 127.0.0.1:12345)
	wsConn *rpcc.Conn  // DevTools WebSocket connection to the browser
	client *cdp.Client // DevTools client using wsConn

	// TODO(hidehiko): Make this field private for better encapsulation.
	Manager *session.Manager // manages connections to multiple targets over wsConn
}

// NewSession establishes a Chrome DevTools Protocol WebSocket connection to the browser.
// This assumes that Chrome listens the debugging port, which means Chrome needs to be
// restarted with a --remote-debugging-port flag.
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
		client:  cl,
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
func (s *Session) Close(ctx context.Context) error {
	err := s.Manager.Close()
	if werr := s.wsConn.Close(); werr != nil {
		// Return the first error. If there already is, just log werr here.
		if err == nil {
			err = werr
		} else {
			testing.ContextLog(ctx, "Failed to close wsConn: ", werr)
		}
	}
	if err != nil {
		return errors.Wrap(err, "failed to close Session")
	}

	return nil
}

// DebugAddrPort returns the addr:port at which Chrome is listening for DevTools connections,
// e.g. "127.0.0.1:38725".
func (s *Session) DebugAddrPort() string {
	return s.addr
}

// CreateTarget opens a new tab displaying the given url.
func (s *Session) CreateTarget(ctx context.Context, url string) (target.ID, error) {
	reply, err := s.client.Target.CreateTarget(ctx, &target.CreateTargetArgs{URL: url})
	if err != nil {
		return "", errors.Wrapf(err, "failed to create a target of %s", url)
	}
	return reply.TargetID, nil
}

// CloseTarget closes the target identified by the given id.
func (s *Session) CloseTarget(ctx context.Context, id target.ID) error {
	if reply, err := s.client.Target.CloseTarget(ctx, &target.CloseTargetArgs{TargetID: id}); err != nil {
		return err
	} else if !reply.Success {
		return errors.New("unknown failure")
	}
	return nil
}

// FindTargets returns the info about Targets, which satisfies the given cond condition.
func (s *Session) FindTargets(ctx context.Context, cond func(*target.Info) bool) ([]*target.Info, error) {
	reply, err := s.client.Target.GetTargets(ctx)
	if err != nil {
		return nil, err
	}

	var matches []*target.Info
	for _, t := range reply.TargetInfos {
		if cond == nil || cond(&t) {
			t := t
			matches = append(matches, &t)
		}
	}
	return matches, nil
}
