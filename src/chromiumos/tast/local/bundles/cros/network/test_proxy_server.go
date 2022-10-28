// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bytes"
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/network/proxy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     TestProxyServer,
		Desc:     "Basic test for proxy.Server",
		Contacts: []string{"acostinas@google.com", "hugobenichi@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

var (
	authRequiredHeader   = []byte("HTTP/1.0 407 Proxy Authentication Required")
	pageMovedHeader      = []byte("HTTP/1.0 301 Moved Permanently")
	noContentHeader      = []byte("HTTP/1.0 204 No Content")
	requestBlockedHeader = []byte("HTTP/1.0 403 Filtered")

	allowedHost = "google.com"
	blockedHost = "ssl.gstatic.com"
	hostname = "support.google.com"
)

// TestProxyServer verifies that proxy.Server with authentication is working as expected.
func TestProxyServer(ctx context.Context, s *testing.State) {
	Server := proxy.NewServer()

	cred := &proxy.AuthCredentials{Username: "user", Password: "test0000"}
	err := Server.Start(ctx, 3128, cred, []string{allowedHost})
	if err != nil {
		s.Fatal("Failed to setup server: ", err)
	}

	// curl request to the local proxy server without auth.
	out, err := testexec.CommandContext(ctx, "curl", "-I", "-x", "http://"+Server.HostAndPort, allowedHost).Output()
	if err != nil {
		s.Error("Curl command without auth failed: ", err)
	} else if !bytes.Contains(out, authRequiredHeader) {
		s.Errorf("Unexpected curl result without auth: got %s; want %s", out, authRequiredHeader)
	}

	// curl request to the local proxy server with credentials.
	out, err = testexec.CommandContext(ctx, "curl", "-I", "-x", "http://user:test0000@"+Server.HostAndPort, allowedHost).Output()
	if err != nil {
		s.Error("Curl command with auth failed: ", err)
	} else if !bytes.Contains(out, pageMovedHeader) && !bytes.Contains(out, noContentHeader) { // Looks like curl requests to google.com may return 204 code (see b/190208108).
		s.Errorf("Unexpected curl result with auth: got %s; want %s", out, pageMovedHeader)
	}

	// curl request to the local proxy server with credentials but blocked hostname.
	out, err = testexec.CommandContext(ctx, "curl", "-I", "-x", "http://user:test0000@"+Server.HostAndPort, blockedHost).Output()
	if err != nil {
		s.Error("Curl command with auth failed with blocked page: ", err)
	} else if !bytes.Contains(out, requestBlockedHeader) {
		s.Errorf("Unexpected curl result with auth: got %s; want %s", out, requestBlockedHeader)
	}

	// Verifies that querying the logs to see if the proxy was used for the connection works as expected.
	used, err := Server.WasProxyUsedForConnection( hostname)
	if err != nil {
		s.Error("Failed to verify connection routed through proxy: ", err)
	} else if used {
		s.Errorf("Unexpected proxy connection: %s was routed to the proxy server", hostname)
	}

	out, err = testexec.CommandContext(ctx, "curl", "-I", "-x", "http://user:test0000@"+Server.HostAndPort, hostname).Output()
	 used, err = Server.WasProxyUsedForConnection(hostname)
 
	if err != nil {
		s.Error("Failed to verify connection routed through proxy: ", err)
	} else if !used {
		s.Errorf("Unexpected direct connection to %s", hostname)
	}

	Server.Stop(ctx)
}
