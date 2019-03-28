// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"io/ioutil"
	"net"

	"testing"
)

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
