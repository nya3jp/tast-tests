// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/network/proxy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtensionInstallForceListWithProxy,
		Desc:         "Checks the installation of a managed extension behind a managed proxy",
		Contacts:     []string{"acostinas@google.com", "cros-networking@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		Timeout:      4 * time.Minute, // There is a longer wait when installing the extension.
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ProxyMode{}, pci.VerifiedValue),
			pci.SearchFlag(&policy.ProxyServer{}, pci.VerifiedValue),
			pci.SearchFlag(&policy.ExtensionInstallForcelist{}, pci.VerifiedValue),
		},
	})
}

func ExtensionInstallForceListWithProxy(ctx context.Context, s *testing.State) {
	// Google keep chrome extension.
	const extensionPolicy = "lpcaedmchfhocbbapmcbpinfpgnhiddi;https://clients2.google.com/service/update2/crx"
	const extensionName = "Google Keep Chrome Extension"
	const chromeWebStoreHostname = "chrome.google.com"
	const extensionURL = "chrome://extensions"

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// Start an HTTP proxy instance on the DUT
	ps := proxy.NewServer()
	defer ps.Stop(ctx)

	err = ps.Start(ctx, 3128, nil, []string{})
	if err != nil {
		s.Fatal("Failed to start a local proxy on the DUT: ", err)
	}

	// Configure the proxy on the DUT via policy to point to the local prox instance
	proxyModePolicy := &policy.ProxyMode{Val: "fixed_servers"}
	proxyServerPolicy := &policy.ProxyServer{Val: fmt.Sprintf("http://%s", ps.HostAndPort)}
	extensionForceListPolicy := &policy.ExtensionInstallForcelist{Val: []string{extensionPolicy}}
	// Update policies.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{proxyModePolicy, proxyServerPolicy, extensionForceListPolicy}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// Check that the extension is installed
	sconn, err := cr.NewConn(ctx, extensionURL)
	if err != nil {
		s.Fatal("Failed to open the extnsion page: ", err)
	}
	defer sconn.Close()

	desc := nodewith.Name(extensionName).Role(role.StaticText)
	ui := uiauto.New(tconn).WithTimeout(3 * time.Minute)

	if err := ui.WaitUntilExists(desc)(ctx); err != nil {
		s.Fatal("Failed to install extension: ", err)
	}

	pu, err := ps.WasProxyUsedForConnection(chromeWebStoreHostname)

	if err != nil {
		s.Fatal("Failed to query proxy logs: ", err)
	}

	if !pu {
		s.Error("Failed to test the extension download behind a proxy")
	}
}
