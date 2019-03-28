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
	cf := func(ctx context.Context) (net.Conn, error) {
		return local, nil
	}
	sdp, err := NewServodProxy(ctx, cf)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := net.Dial("tcp", sdp.LocalAddress())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		bytesWritten, err := conn.Write(expected)
		if err != nil {
			t.Fatal(err)
		}
		if bytesWritten != len(expected) {
			t.Errorf("conn wrote %d bytes, expected %d", bytesWritten, len(expected))
		}
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

func TestHandleProxyConnection(t *testing.T) {
	local, remote := net.Pipe()
	defer local.Close()
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
