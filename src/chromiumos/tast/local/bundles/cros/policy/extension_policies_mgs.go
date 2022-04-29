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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtensionPoliciesMGS,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify that extension policies reach the app on Managed Guest Session",
		Contacts: []string{
			"sergiyb@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

const (
	enterpriseTestExtensionID   = "dbbinhebhbmlbjnjpeiledcefofbelcl"
	enterpriseTestExtensionName = "Enterprise Verified Access Test Bed"
	extensionVersion            = "3.1.28"
	defaultProxyServerURL       = "https://test-proxy-server-1.example.com/"
	updatedProxyServerURL       = "https://test-proxy-server-2.example.com/"
	extensionPolicyTemplate     = `{"ProxyUrl":{"Value":"%s"}}`
)

func ExtensionPoliciesMGS(ctx context.Context, s *testing.State) {
	// TODO(b/200987907): Convert this to an E2E test using a real account with
	// pre-configured policies. The challenge is to modify policies on DPanel
	// from the test and make sure that they are reset back if test crashes.
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	pb := policy.NewBlob()
	pb.ExtensionPM = make(policy.BlobPolicyMap)
	pb.ExtensionPM[enterpriseTestExtensionID] = json.RawMessage(
		fmt.Sprintf(extensionPolicyTemplate, defaultProxyServerURL))

	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
		mgs.ExternalPolicyBlob(pb),
		mgs.AddPublicAccountPolicies(mgs.MgsAccountID, []policy.Policy{
			&policy.ExtensionInstallForcelist{Val: []string{enterpriseTestExtensionID}},
		}),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome on Signin screen with default MGS account: ", err)
	}
	defer func() {
		if err := mgs.Close(ctx); err != nil {
			s.Fatal("Failed close MGS: ", err)
		}
	}()

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// This is needed so that browser is visible and we can open extension popup below.
	if _, err := cr.NewConn(ctx, ""); err != nil {
		s.Fatal("Failed to open newtab page: ", err)
	}

	ui := uiauto.New(tconn)
	extensionButtonFinder := nodewith.Name(enterpriseTestExtensionName).Role(role.Button)
	proxyServiceFieldFinder := nodewith.Name("Proxy Service").Role(role.TextField)
	proxyServiceURLFinder := nodewith.Ancestor(proxyServiceFieldFinder).Role(role.InlineTextBox).Name(defaultProxyServerURL)
	if err = uiauto.Combine("open extension popup and wait for default Proxy Service URL",
		ui.LeftClick(nodewith.Name("Extensions").Role(role.PopUpButton)),
		ui.WaitForLocation(extensionButtonFinder),
		ui.LeftClick(extensionButtonFinder),
		ui.WaitForLocation(proxyServiceURLFinder),
	)(ctx); err != nil {
		s.Fatal("Failed to open extension popup and find default Proxy Service URL: ", err)
	}

	pb.ExtensionPM[enterpriseTestExtensionID] = json.RawMessage(
		fmt.Sprintf(extensionPolicyTemplate, updatedProxyServerURL))
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh policies: ", err)
	}

	proxyServiceURLFinder = proxyServiceURLFinder.Name(updatedProxyServerURL)
	if err := ui.WaitForLocation(proxyServiceURLFinder)(ctx); err != nil {
		s.Fatal("Failed while waiting for updated Proxy Service URL: ", err)
	}
}
