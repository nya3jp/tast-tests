// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"io/ioutil"
	"net"

	"testing"
)

func TestProxyConnection(t *testing.T) {
	local, remote := net.Pipe()
	go proxyConnection(local, remote)

	expected := []byte("rutabaga")
	go func() {
		local.Write(expected)
		local.Close()
	}()

	actual, err := ioutil.ReadAll(remote)
	if err != nil {
		t.Errorf("reading remote conn failed: %v", err)
		return
	}

	actualStr := string(actual)
	expectedStr := string(expected)
	if actualStr != expectedStr {
		t.Errorf("proxyConnection(%q) returned %q; want %q", expectedStr, actualStr, expectedStr)
	}
}
