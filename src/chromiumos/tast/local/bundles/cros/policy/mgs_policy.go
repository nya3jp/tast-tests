// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MgsPolicy,
		Desc: "Sample test stating MGS and applying HighContrastEnabled policy",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"chromeos-commercial-identity@google.com",
		},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func MgsPolicy(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepState(),
		// chrome.ExtraArgs("--disable-policy-key-verification"),
	)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	defer func(ctx context.Context) {
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	accountID := "foo@bar.com"
	accountType := policy.AccountTypePublicSession

	policies := []policy.Policy{
		&policy.DeviceLocalAccounts{
			Val: []policy.DeviceLocalAccountInfo{
				{
					AccountID:   &accountID,
					AccountType: &accountType,
				},
			},
		},
	}

	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(policies)

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	// Close the previous Chrome instance.
	if err := cr.Close(ctx); err != nil {
		s.Fatal("Failed to close Chrome connection: ", err)
	}

	// Restart Chrome, forcing Devtools to be available on the login screen.
	cr, err = chrome.New(ctx,
		chrome.DeferLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)

	if err != nil {
		s.Fatal("Chrome restart failed: ", err)
	}

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, testConn)

	// testing.Sleep(ctx, 3*time.Second)

	mgsArrowBtn := nodewith.ClassName("ArrowButtonView")
	confirmLoginArrow := nodewith.ClassName("ArrowButtonView").Name("Log in")
	ui := uiauto.New(testConn)
	if err := uiauto.Combine("Login to public managed session",
		ui.WaitUntilExists(mgsArrowBtn),
		ui.LeftClick(mgsArrowBtn),
		ui.LeftClick(confirmLoginArrow),
	)(ctx); err != nil {
		s.Fatal("Failed to open menu with local accounts: ", err)
	}

	pb.AddPublicAccountPolicy(accountID, &policy.HighContrastEnabled{
		Val: true,
	})
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies in MGS mode: ", err)
	}

	script := `(async () => {
		let result = await tast.promisify(tast.bind(chrome.accessibilityFeatures['highContrast'], "get"))({});
		return result.value;
	  })()`

	var policyValue bool
	if err := testConn.Eval(ctx, script, &policyValue); err != nil {
		s.Fatalf("Failed to retrieve highContrast enabled value: %s", err)
	}
	// TODO: this fails -> got false deprite seeing policy applied.
	if policyValue != true {
		s.Errorf("Unexpected value of chrome.accessibilityFeatures[highContrast]: got %t; want true", policyValue)
	}
}
