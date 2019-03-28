// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"io/ioutil"
	"net"

	"testing"
)

func TestNewServodProxy(t *testing.T) {
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()

	expected := []byte("rutabaga")
	ctx := context.TODO()

	// Set up a "remote" TCP listener.
	/*
		remoteListener, err := net.Listen("tcp", ":0")
		defer remoteListener.Close()
		if err != nil {
			t.Fatal(err)
		}
		remoteTCP, err := net.Dial("tcp", remoteListener.Addr())
		defer remoteTCP.Close()
		if err != nil {
			t.Fatal(err)
		}
	*/

	cf := func() (net.Conn, error) {
		return remote, nil
	}
	sdp, err := NewServodProxy(ctx, cf)
	if err != nil {
		t.Fatal(err)
	}

	go func(proxy *ServodProxy, lc net.Conn) {
		localTCP, err := net.Dial("tcp", proxy.LocalAddress())
		if err != nil {
			t.Fatal(err)
		}
		bytesWritten, err := localTCP.Write(expected)
		if err != nil {
			t.Fatal(err)
		}
		if bytesWritten != len(expected) {
			t.Errorf("conn wrote %d bytes, expected %d", bytesWritten, len(expected))
		}
		if localTCP.Close(); err != nil {
			t.Fatal(err)
		}
		if lc.Close(); err != nil {
			t.Fatal(err)
		}
	}(sdp, local)

	// TODO(CL): Still need to work out connection/concurrency issues...
	actual, err := ioutil.ReadAll(remote)
	if err != nil {
		t.Fatalf("reading remote conn failed: %v", err)
	}
	actualStr := string(actual)
	expectedStr := string(expected)
	if actualStr != expectedStr {
		t.Errorf("ServodProxy wrote %q; want %q", actualStr, expectedStr)
	}
}

/*
func TestHandleProxyConnection(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()

	go handleProxyConnection(local, remote)

	expected := []byte("rutabaga")
	go func() {
		local.Write(expected)
		local.Close()
	}()

	actual, err := ioutil.ReadAll(remote)
	if err != nil {
		t.Fatalf("reading remote conn failed: %v", err)
	}

	actualStr := string(actual)
	expectedStr := string(expected)
	if actualStr != expectedStr {
		t.Errorf("proxyConnection() with input %q copied %q; want %q", expectedStr, actualStr, expectedStr)
	}
}
*/
