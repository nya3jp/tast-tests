// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"regexp"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProxyResolutionService,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that the ProxyResolutionService in Chrome works as expected",
		Contacts:     []string{"acostinas@google.com", "chromeos-commercial-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func ProxyResolutionService(ctx context.Context, s *testing.State) {
	const (
		dbusName   = "org.chromium.NetworkProxyService"
		dbusPath   = "/org/chromium/NetworkProxyService"
		dbusMethod = "org.chromium.NetworkProxyServiceInterface.ResolveProxy"
	)

	_, err := chrome.New(
		ctx,
		chrome.ExtraArgs("--proxy-server=http://localhost:8888", "--enable-features=SystemProxyForSystemServices"))
	if err != nil {
		s.Fatal("Failed to start Chrome with proxy config: ", err)
	}

	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	var state string
	var errorMsg string
	// Call the proxy resolution service without the SystemProxyOverride option.
	if err := obj.CallWithContext(ctx, dbusMethod, 0, "https://google.com").Store(&state, &errorMsg); err != nil {
		s.Error("Failed to get the proxy: ", err)
	}

	const pacProxy = "PROXY localhost:8888"
	if state != pacProxy {
		s.Fatalf("Unexpected proxy resolution result: got %s; want %s", state, pacProxy)
	}

	// Call the proxy resolution service with SystemProxyOverride=Default. Since system-proxy is enabled via flag (and not policy),
	// the default option means that system-proxy will not appear in the PAC-style list of returned proxies.
	systemProxyOverrideOption := 0
	if err := obj.CallWithContext(ctx, dbusMethod, 0, "https://google.com", systemProxyOverrideOption).Store(&state, &errorMsg); err != nil {
		s.Error("Failed to get the proxy: ", err)
	}

	if state != pacProxy {
		s.Fatalf("Unexpected proxy resolution result: got %s; want %s", state, pacProxy)
	}

	// Call the proxy resolution service with SystemProxyOverride=OptIn.
	systemProxyOverrideOption = 1
	if err := obj.CallWithContext(ctx, dbusMethod, 0, "https://google.com", systemProxyOverrideOption).Store(&state, &errorMsg); err != nil {
		s.Error("Failed to get the proxy: ", err)
	}

	// The system-proxy daemon has an address in the 100.115.92.0/24 subnet (assigned by patchpanel) and listens on port 3128.
	const pacProxyWithSystemProxy = "PROXY 100.115.92.[0-9]+:3128; PROXY localhost:8888"
	proxyRegex := regexp.MustCompile(pacProxyWithSystemProxy)
	if !proxyRegex.Match([]byte(state)) {
		s.Fatalf("Unexpected proxy resolution result: got %s; want %s", state, pacProxyWithSystemProxy)
	}

	// Call the proxy resolution service with SystemProxyOverride=OptOut.
	systemProxyOverrideOption = 2
	if err := obj.CallWithContext(ctx, dbusMethod, 0, "https://google.com", systemProxyOverrideOption).Store(&state, &errorMsg); err != nil {
		s.Error("Failed to get the proxy: ", err)
	}

	// The address of the system-proxy should not appear in the PAC-style list of returned proxies.
	if state != pacProxy {
		s.Fatalf("Unexpected proxy resolution result: got %s; want %s", state, pacProxy)
	}

	// Call the proxy resolution service with an invalid value for SystemProxyOverride.
	systemProxyOverrideOption = 44
	if err := obj.CallWithContext(ctx, dbusMethod, 0, "https://google.com", systemProxyOverrideOption).Store(&state, &errorMsg); err != nil {
		s.Error("Failed to get the proxy: ", err)
	}

	// The result should be the same as when using the default value, i.e. system-proxy should not appear in the returned value.
	if state != pacProxy {
		s.Fatalf("Unexpected proxy resolution result: got %s; want %s", state, pacProxy)
	}
}
