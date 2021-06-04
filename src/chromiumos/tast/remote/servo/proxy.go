// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"net"
	"strconv"
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

func splitHostPort(servoHostPort string) (string, int, int, error) {
	host := "localhost"
	port := 9999
	sshPort := 22

	hostport := servoHostPort
	sshParts := strings.SplitN(hostport, ":ssh:", 2)
	if len(sshParts) == 2 {
		hostport = sshParts[0]
		var err error
		if sshPort, err = strconv.Atoi(sshParts[1]); err != nil {
			return "", 0, 0, errors.Wrap(err, "parsing servo host ssh port")
		}
		if sshPort <= 0 {
			return "", 0, 0, errors.New("invalid servo host ssh port")
		}
	}

	// The port starts after the last colon.
	i := strings.LastIndexByte(hostport, ':')
	if i >= 0 {
		if hostport[0] == '[' {
			// Expect the first ']' just before the last ':'.
			end := strings.IndexByte(hostport, ']')
			if end < 0 {
				return "", 0, 0, errors.New("missing ']' in address")
			}
			switch end + 1 {
			case len(hostport): // No port
				if hostport[1:end] != "" {
					host = hostport[1:end]
				}
				return host, port, sshPort, nil
			case i: // ] before :
				if hostport[1:end] != "" {
					host = hostport[1:end]
				}
			default:
				return "", 0, 0, errors.New("servo arg must be of the form hostname:9999 or hostname:9999:ssh:22 or [::1]:9999")
			}
		} else {
			if hostport[:i] != "" {
				host = hostport[:i]
			}
			if strings.IndexByte(host, ':') >= 0 {
				return "", 0, 0, errors.New("unexpected colon in hostname")
			}
		}
		var err error
		if port, err = strconv.Atoi(hostport[i+1:]); err != nil {
			return "", 0, 0, errors.Wrap(err, "parsing servo port")
		}
		if port <= 0 {
			return "", 0, 0, errors.New("invalid servo port")
		}
	} else if hostport != "" {
		host = hostport
	}
	return host, port, sshPort, nil
}

// NewProxy returns a Proxy object for communicating with the servod instance at spec,
// which can be blank (defaults to localhost:9999:ssh:22) or a hostname (defaults to hostname:9999:ssh:22)
// or a host:port (ssh port defaults to 22) or to fully qualify everything host:port:ssh:sshport.
//
// You can also use IPv4 addresses as the hostnames, or IPv6 addresses in square brackets [::1].
//
// If you are using ssh port forwarding, please note that the host and ssh port will be evaluated locally,
// but the servo port should be the real servo port on the servo host.
// So if you used the ssh command `ssh -L 2223:localhost:22 -L 2222:${DUT_HOSTNAME?}:22 root@${SERVO_HOSTNAME?}`
// then you would start tast with `tast run --var=servo=localhost:${SERVO_PORT?}:ssh:2223 localhost:2222 firmware.Config*`
//
// If the instance is not running on the local system, an SSH connection will be opened
// to the host running servod and servod connections will be forwarded through it.
// keyFile and keyDir are used for establishing the SSH connection and should
// typically come from dut.DUT's KeyFile and KeyDir methods.
func NewProxy(ctx context.Context, servoHostPort, keyFile, keyDir string) (newProxy *Proxy, retErr error) {
	var pxy Proxy
	toClose := &pxy
	defer func() {
		if toClose != nil {
			toClose.Close(ctx)
		}
	}()

	host, port, sshPort, err := splitHostPort(servoHostPort)
	if err != nil {
		return nil, err
	}
	// If the servod instance isn't running locally, assume that we need to connect to it via SSH.
	if (host != "localhost" && host != "127.0.0.1" && host != "::1") || sshPort != 22 {
		// First, create an SSH connection to the remote system running servod.
		sopt := ssh.Options{
			KeyFile:        keyFile,
			KeyDir:         keyDir,
			ConnectTimeout: proxyTimeout,
			WarnFunc:       func(msg string) { testing.ContextLog(ctx, msg) },
			Hostname:       net.JoinHostPort(host, fmt.Sprint(sshPort)),
			User:           "root",
		}
		testing.ContextLogf(ctx, "Opening Servo SSH connection to %s", sopt.Hostname)
		var err error
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
		pxy.fwd, err = pxy.hst.NewForwarder("localhost:0", fmt.Sprintf("localhost:%d", port),
			func(err error) { testing.ContextLog(ctx, "Got servo forwarding error: ", err) })
		if err != nil {
			return nil, err
		}
		var portstr string
		if host, portstr, err = net.SplitHostPort(pxy.fwd.ListenAddr().String()); err != nil {
			return nil, err
		}
		if port, err = strconv.Atoi(portstr); err != nil {
			return nil, errors.Wrap(err, "parsing forwarded servo port")
		}
	}

	testing.ContextLogf(ctx, "Connecting to servod at %s:%d", host, port)
	pxy.svo, err = New(ctx, host, port)
	if err != nil {
		return nil, err
	}
	toClose = nil // disarm cleanup
	return &pxy, nil
}

// logServoStatus logs the current servo status from the servo host.
func logServoStatus(ctx context.Context, hst *ssh.Conn, port int) {
	// Check if servod is running of the servo host.
	out, err := hst.Command("servodtool", "instance", "show", "-p", fmt.Sprint(port)).CombinedOutput(ctx)
	if err != nil {
		testing.ContextLogf(ctx, "Servod process is not initialized on the servo-host: %v: %v", err, string(out))
		return
	}
	testing.ContextLogf(ctx, "Servod instance is running on port %v of the servo host", port)
	// Check if servod is busy.
	if out, err = hst.Command("dut-control", "-p ", fmt.Sprint(port), "serialname").CombinedOutput(ctx); err != nil {
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
