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
		Desc:     "Test proxy server",
		Contacts: []string{"acostinas@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

const (
	authRequiredHeader = "HTTP/1.0 407 Proxy Authentication Required"
	pageMovedHeader    = "HTTP/1.0 301 Moved Permanently"
)

func TestProxyServer(ctx context.Context, s *testing.State) {
	proxyServer := proxy.NewProxyServer()

	cred := &proxy.AuthCredentials{Username: "user", Password: "test0000"}
	err := proxyServer.StartServer(ctx, 3128, cred)

	if err != nil {
		s.Fatal("Failed to setup server")
	}

	// curl request to the local proxy server without auth
	out, err := testexec.CommandContext(ctx, "curl", "-I", "-x", "http://"+proxyServer.HostAndPort, "google.com").Output()
	if err != nil {
		s.Error("Curl command failed")
	}
	response := string(out)
	if !strings.Contains(response, authRequiredHeader) {
		s.Error("Unexpected curl result: " + response)
	}

	// curl request to the local proxy server with credentials
	out, err = testexec.CommandContext(ctx, "curl", "-I", "-x", "http://user:test0000@"+proxyServer.HostAndPort, "google.com").Output()
	if err != nil {
		s.Error("Curl command failed")
	}
	response = string(out)
	if !strings.Contains(response, pageMovedHeader) {
		s.Error("Unexpected curl result: " + response)
	}

	proxyServer.StopServer(ctx)
}
