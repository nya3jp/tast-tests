// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"io"
	"net"

	"chromiumos/tast/host"
)

// ConnFunc is used by ServodProxy to access the remote device running servod.
type ConnFunc = func(ctx context.Context) (net.Conn, error)

// ServodProxy represents a TCP proxy for servod XML-RPC commands.
type ServodProxy struct {
	localPort     int
	localListener net.Listener
}

// LocalPort returns the port ServodProxy is running on.
func (sdp *ServodProxy) LocalPort() int {
	return sdp.localPort
}

// Close stops the ServodProxy.
func (sdp *ServodProxy) Close() error {
	return sdp.localListener.Close()
}

// NewServodProxy sets up SSH TCP tunneling to a remote machine running servod.
func NewServodProxy(ctx context.Context, cf ConnFunc) (*ServodProxy, error) {
	sdp := ServodProxy{}

	// Start local proxy server on the first available port.
	localListener, err := net.Listen("tcp", ":0")
	sdp.localListener = localListener
	if err != nil {
		return nil, err
	}
	sdp.localPort = localListener.Addr().(*net.TCPAddr).Port

	// Handle incoming connections
	go func() {
		for {
			local, err := localListener.Accept()
			if err != nil {
				// TODO(jeffcarp) better expose this error.
			}

			remote, err := cf(ctx)
			if err != nil {
				// TODO(jeffcarp) better expose this error.
			}

			proxyConnection(local, remote)
		}
	}()

	return &sdp, nil
}

// NewSSHServodProxy returns a new ServodProxy using TCP-over-SSH tunneling.
func NewSSHServodProxy(ctx context.Context, ssh *host.SSH, remotePort int) (*ServodProxy, error) {
	remoteAddr := net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: remotePort, Zone: ""}
	return NewServodProxy(ctx, func(ctx context.Context) (net.Conn, error) {
		return ssh.DialTCP(&remoteAddr)
	})
}

// Copy data between a local and a remote connection.
func proxyConnection(local, remote net.Conn) {
	chDone := make(chan bool)
	defer local.Close()
	defer remote.Close()

	go func() {
		_, err := io.Copy(local, remote)
		if err != nil {
			// TODO(jeffcarp) better expose this error.
		}
		chDone <- true
	}()

	go func() {
		_, err := io.Copy(remote, local)
		if err != nil {
			// TODO(jeffcarp) better expose this error.
		}
		chDone <- true
	}()

	<-chDone
}
