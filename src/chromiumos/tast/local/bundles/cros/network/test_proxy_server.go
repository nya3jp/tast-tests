// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strings"

	"chromiumos/tast/local/bundles/cros/network/proxy"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     TestProxyServer,
		Desc:     "Sanity test for proxy.Server",
		Contacts: []string{"acostinas@google.com", "hugobenichi@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

const (
	authRequiredHeader = "HTTP/1.0 407 Proxy Authentication Required"
	pageMovedHeader    = "HTTP/1.0 301 Moved Permanently"
)

// TestProxyServer verifies that proxy.Server with authentication is working as expected.
func TestProxyServer(ctx context.Context, s *testing.State) {
	Server := proxy.NewServer()

	cred := &proxy.AuthCredentials{Username: "user", Password: "test0000"}
	err := Server.Start(ctx, 3128, cred)

	if err != nil {
		s.Fatal("Failed to setup server: ", err)
	}

	// curl request to the local proxy server without auth.
	out, err := testexec.CommandContext(ctx, "curl", "-I", "-x", "http://"+Server.HostAndPort, "google.com").Output()
	if err != nil {
		s.Error("Curl command failed: ", err)
	}
	response := string(out)
	// Expect that the server responds that authentication is missing.
	if !strings.Contains(response, authRequiredHeader) {
		s.Error("Unexpected curl result: " + response)
	}

	// curl request to the local proxy server with credentials.
	out, err = testexec.CommandContext(ctx, "curl", "-I", "-x", "http://user:test0000@"+Server.HostAndPort, "google.com").Output()
	if err != nil {
		s.Error("Curl command failed: ", err)
	}
	response = string(out)
	// Expect that the connection was successful.
	if !strings.Contains(response, pageMovedHeader) {
		s.Error("Unexpected curl result: " + response)
	}

	Server.Stop(ctx)
}
