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
	"chromiumos/tast/testing"
)

type ConnFunc = func(ctx context.Context, remoteAddr string)

type ServodProxy struct {}

// NewServodProxy sets up SSH TCP tunneling to a remote machine running servod.
func NewServodProxy(ctx context.Context, localPort, remotePort int, ssh *host.SSH, s *testing.State) (*ServodProxy, error) {
	sdp := ServodProxy{}

	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	remoteAddr := net.TCPAddr{IP:net.IPv4(127, 0, 0, 1), Port:remotePort, Zone:""}

	// Start local proxy server.
	localListener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return nil, err
	}

	// Handle incoming connections
	go func() {
		defer localListener.Close()
		for {
			local, err := localListener.Accept()
			s.Log("DEBUG: GOT CONNECTION!!!!", local)
			if err != nil {
				// TODO(CL) do something...
				s.Log("ERR:", err)
				return
			}

			remote, err := ssh.DialTCP(&remoteAddr)
			if err != nil {
				// TODO(CL) do something...
				s.Log("ERR:", err)
				return
			}
			defer remote.Close()

			proxyConnection(local, remote)
		}
	}()

	return &sdp, nil
}

// Copy data between a local and a remote connection.
func proxyConnection(local net.Conn, remote net.Conn) {
	chDone := make(chan bool)
	defer local.Close()

	go func() {
		_, err := io.Copy(local, remote)
		if err != nil {
			// TODO(CL) do something...
		}
		chDone <- true
	}()

	go func() {
		_, err := io.Copy(remote, local)
		if err != nil {
			// TODO(CL) do something...
		}
		chDone <- true
	}()

	<-chDone
}
