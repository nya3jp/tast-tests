// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"fmt"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	extenID             = "dbbinhebhbmlbjnjpeiledcefofbelcl"
	extenName           = "Enterprise Verified Access Test Bed"
	extenURL            = "https://chrome.google.com/webstore/detail/enterprise-verified-acces/dbbinhebhbmlbjnjpeiledcefofbelcl"
	extenVersion        = "3.1.28"
	defaultProxyServURL = "https://test-proxy-server-1.example.com/"
	updatedProxyServURL = "https://test-proxy-server-2.example.com/"
	extenPolicyTemplate = `{"ProxyUrl":{"Value":"%s"}}`
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtensionPoliciesRegularUser,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify that extension policies reach the app in a regular user session",
		Contacts: []string{
			"emaxx@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
	})
}

func ExtensionPoliciesRegularUser(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	pb := policy.NewBlob()
	pb.ExtensionPM = make(policy.BlobPolicyMap)
	pb.ExtensionPM[extenID] = json.RawMessage(fmt.Sprintf(extenPolicyTemplate, defaultProxyServURL))
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh policies: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Install the test extension by visiting the Web Store page.
	cwsApp := cws.App{Name: extenName, URL: extenURL}
	if err := cws.InstallApp(ctx, cr, tconn, cwsApp); err != nil {
		s.Fatal("Failed to install test extension: ", err)
	}
	defer cws.UninstallApp(ctx, cr, tconn, cwsApp)

	// This is needed so that browser is visible and we can open extension popup below.
	if _, err := cr.NewConn(ctx, ""); err != nil {
		s.Fatal("Failed to open newtab page: ", err)
	}

	// Check the policy-configured URL appears in the extension UI.
	ui := uiauto.New(tconn)
	extensionButtonFinder := nodewith.Name(extenName).Role(role.Button)
	proxyServiceFieldFinder := nodewith.Name("Proxy Service").Role(role.TextField)
	proxyServiceURLFinder := nodewith.Ancestor(proxyServiceFieldFinder).Role(role.InlineTextBox).Name(defaultProxyServURL)
	if err = uiauto.Combine("open extension popup and wait for default Proxy Service URL",
		ui.LeftClick(nodewith.Name("Extensions").Role(role.PopUpButton)),
		ui.WaitForLocation(extensionButtonFinder),
		ui.LeftClick(extensionButtonFinder),
		ui.WaitForLocation(proxyServiceURLFinder),
	)(ctx); err != nil {
		s.Fatal("Failed to open extension popup and find default Proxy Service URL: ", err)
	}

	// Update the extension's policy.
	pb.ExtensionPM[extenID] = json.RawMessage(fmt.Sprintf(extenPolicyTemplate, updatedProxyServURL))
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh policies: ", err)
	}

	// Check the update is reflected in the extension UI.
	proxyServiceURLFinder = proxyServiceURLFinder.Name(updatedProxyServURL)
	if err := ui.WaitForLocation(proxyServiceURLFinder)(ctx); err != nil {
		s.Fatal("Failed while waiting for updated Proxy Service URL: ", err)
	}
}
