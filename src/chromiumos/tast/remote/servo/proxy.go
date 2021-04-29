// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"net"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const proxyTimeout = 10 * time.Second // max time for establishing SSH connection

// Proxy wraps a Servo object and forwards connections to the servod instance
// over SSH if needed.
type Proxy struct {
	svo *Servo
	hst *ssh.Conn      // nil if servod is running locally
	fwd *ssh.Forwarder // nil if servod is running locally
}

// NewProxy returns a Proxy object for communicating with the servod instance at spec,
// which takes the same form passed to New (i.e. "host:port").
//
// If the instance is not running on the local system, an SSH connection will be opened
// to the host running servod and servod connections will be forwarded through it.
// keyFile and keyDir are used for establishing the SSH connection and should
// typically come from dut.DUT's KeyFile and KeyDir methods.
func NewProxy(ctx context.Context, spec, keyFile, keyDir string) (newProxy *Proxy, retErr error) {
	var pxy Proxy
	toClose := &pxy
	defer func() {
		if toClose != nil {
			toClose.Close(ctx)
		}
	}()

	// If the servod instance isn't running locally, assume that we need to connect to it via SSH.
	if !strings.HasPrefix(spec, "localhost:") && !strings.HasPrefix(spec, "127.0.0.1:") {
		hostname, port, err := net.SplitHostPort(spec)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse %q", spec)
		}

		// First, create an SSH connection to the remote system running servod.
		sopt := ssh.Options{
			KeyFile:        keyFile,
			KeyDir:         keyDir,
			ConnectTimeout: proxyTimeout,
			WarnFunc:       func(msg string) { testing.ContextLog(ctx, msg) },
		}
		// Use the default SSH username and port.
		if err := ssh.ParseTarget(hostname, &sopt); err != nil {
			return nil, err
		}

		testing.ContextLogf(ctx, "Opening SSH connection to %s", sopt.Hostname)
		if pxy.hst, err = ssh.New(ctx, &sopt); err != nil {
			return nil, err
		}
		defer func() {
			if retErr != nil {
				logServoStatus(ctx, pxy.hst, port)
			}
		}()
		// Next, forward a local port over the SSH connection to the servod port.
		testing.ContextLog(ctx, "Creating forwarded connection to port ", port)
		pxy.fwd, err = pxy.hst.NewForwarder("localhost:0", "localhost:"+port,
			func(err error) { testing.ContextLog(ctx, "Got servo forwarding error: ", err) })
		if err != nil {
			return nil, err
		}
		spec = pxy.fwd.LocalAddr().String()
	}

	testing.ContextLog(ctx, "Connecting to servod at ", spec)
	var err error
	pxy.svo, err = New(ctx, spec)
	if err != nil {
		return nil, err
	}
	toClose = nil // disarm cleanup
	return &pxy, nil
}

// logServoStatus logs the current servo status from the servo host.
func logServoStatus(ctx context.Context, hst *ssh.Conn, port string) {
	// Check if servod is running of the servo host.
	out, err := hst.Command("servodtool", "instance", "show", "-p", port).CombinedOutput(ctx)
	if err != nil {
		testing.ContextLogf(ctx, "Servod process is not initialized on the servo-host: %v: %v", err, string(out))
		return
	}
	testing.ContextLogf(ctx, "Servod instance is running on port %v of the servo host", port)
	// Check if servod is busy.
	if out, err = hst.Command("dut-control", "-p ", port, "serialname").CombinedOutput(ctx); err != nil {
		testing.ContextLogf(ctx, "The servod is not responsive or busy: %v: %v", err, string(out))
		return
	}
	testing.ContextLog(ctx, "Servod is responsive on the host and can provide information about serialname: ", string(out))
}

// Close closes the proxy's SSH connection if present.
func (p *Proxy) Close(ctx context.Context) {
	if p.svo != nil {
		p.svo.Close(ctx)
		p.svo = nil
	}
	if p.fwd != nil {
		p.fwd.Close()
		p.fwd = nil
	}
	if p.hst != nil {
		p.hst.Close(ctx)
		p.hst = nil
	}
}

// Servo returns the proxy's encapsulated Servo object.
func (p *Proxy) Servo() *Servo { return p.svo }
