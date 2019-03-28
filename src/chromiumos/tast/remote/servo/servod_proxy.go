// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"io"
	"net"

	"chromiumos/tast/host"
)

// ConnFunc is called by ServodProxy to establish a new connection to the
// remote device running servod.
type ConnFunc = func(ctx context.Context) (net.Conn, error)

// ServodProxy represents a TCP proxy for servod XML-RPC commands.
type ServodProxy struct {
	localPort     int
	localListener net.Listener
}

// LocalAddress returns the local address ServodProxy is running on.
func (sdp *ServodProxy) LocalAddress() string {
	return fmt.Sprintf("127.0.0.1:%d", sdp.localPort)
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

			handleProxyConnection(local, remote)
		}
	}()

	return &sdp, nil
}

// NewSSHServodProxy returns a new ServodProxy using TCP-over-SSH tunneling.
func NewSSHServodProxy(ctx context.Context, ssh *host.SSH, remoteAddr string) (*ServodProxy, error) {
	addr, err := net.ResolveTCPAddr("tcp", remoteAddr)
	if err != nil {
		return nil, err
	}
	return NewServodProxy(ctx, func(context.Context) (net.Conn, error) {
		// TODO(jeffcarp): Update DialTCP to honor context deadlines.
		return ssh.DialTCP(addr)
	})
}

// Copy data between a local and a remote connection. This is executed via a
// goroutine to service a connection opened by ServoProxy.
func handleProxyConnection(local, remote net.Conn) {
	chDone := make(chan bool)
	defer local.Close()
	defer remote.Close()

	go func() {
		if _, err := io.Copy(local, remote); err != nil {
			// TODO(jeffcarp) better expose this error.
		}
		chDone <- true
	}()

	go func() {
		if _, err := io.Copy(remote, local); err != nil {
			// TODO(jeffcarp) better expose this error.
		}
		chDone <- true
	}()

	<-chDone
}
