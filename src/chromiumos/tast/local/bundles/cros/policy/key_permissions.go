// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyPermissions,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of KeyPermissions policy: verify that The admin can provide permission to specific 3rd-party extensions to receive access to platform managed certificates by explictly listing the extension in the policy, otherwise, the extensions cannot have access to the certificates",
		Contacts: []string{
			"mgawad@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:    "ash",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Timeout: 40 * time.Second,
	})
}

func KeyPermissions(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	const (
		extensionID          = "hoppbgdeajkagempifacalpdapphfoai" // PlatformKeys Test Extension
		extensionMainPageURL = "chrome-extension://" + extensionID + "/main.html"
	)
	extensionInstallForcelistPolicyValue := &policy.ExtensionInstallForcelist{Val: []string{extensionID}}

	for _, param := range []struct {
		name         string
		expectAccess bool                   // expectAccess defines if the extension should access the platform-managed certificates.
		value        *policy.KeyPermissions // value is the value of the policy.
	}{
		{
			name:         "has_access",
			expectAccess: true,
			value:        &policy.KeyPermissions{Val: map[string]*policy.KeyPermissionsValue{extensionID: &policy.KeyPermissionsValue{AllowCorporateKeyUsage: true}}},
		},
		{
			name:         "no_access",
			expectAccess: false,
			value:        &policy.KeyPermissions{Val: map[string]*policy.KeyPermissionsValue{extensionID: &policy.KeyPermissionsValue{AllowCorporateKeyUsage: false}}},
		},
		{
			name:         "unset",
			expectAccess: false,
			value:        &policy.KeyPermissions{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value, extensionInstallForcelistPolicyValue}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			conn, br, closeBrowser, err := browserfixt.SetUpWithURL(ctx, s.FixtValue(), s.Param().(browser.Type), "")
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer conn.Close()

			// Verify policies are set to the expected values.
			if err := verifyPoliciesValue(ctx, conn, param.expectAccess); err != nil {
				s.Fatal("Verify policies value failed: ", err)
			}

			// Force Chrome to update all extensions to make sure our extension is installed.
			if err := forceUpdateChromeExtensions(ctx, br, conn); err != nil {
				s.Fatal("Failed to force update chrome extensions: ", err)
			}

			if err := conn.Navigate(ctx, extensionMainPageURL); err != nil {
				s.Fatalf("Failed to connect to the %s page: %s", extensionMainPageURL, err)
			}

			var hasAccess bool
			// If the extension main page has no access to the PlatformKeys API function, there will be bindings.
			if err := conn.Eval(ctx, `chrome.enterprise !== undefined && chrome.enterprise.platformKeys !== undefined`, &hasAccess); err != nil {
				s.Fatal("Failed to execute Javascript: ", err)
			}
			if hasAccess != param.expectAccess {
				s.Fatalf("Extension's access to chrome.enterprise.platformKeys API=%t, expected=%t", hasAccess, param.expectAccess)
			}
		})
	}
}

// verifyPoliciesValue verifies policies are set to the expected values in chrome://policy/.
func verifyPoliciesValue(ctx context.Context, conn *chrome.Conn, expectAccess bool) error {
	if err := conn.Navigate(ctx, "chrome://policy/"); err != nil {
		return errors.Wrap(err, "failed to navigate to chrome://policy/ page")
	}

	var allowCorporateKeyUsage bool

	if err := conn.Eval(ctx, `new Promise((resolve, reject) => {
		let extensionID = "hoppbgdeajkagempifacalpdapphfoai";
		var allowCorporateKeyUsage = false;
		let arr = document.getElementsByClassName("policy row");
		for (var i = 0; i < arr.length; i++) {
			let nameElements = arr[i].getElementsByClassName("name");
			if (nameElements.length === 0 || nameElements[0].getElementsByTagName("span").length === 0)
				continue;
			let policyName = nameElements[0].getElementsByTagName("span")[0].innerText;
			let policyValue = arr[i].getElementsByClassName("value")[0].innerText;
			if (policyName === "ExtensionInstallForcelist") {
				if (policyValue !== extensionID) {
					reject(new Error("Unexpected policy value for ExtensionInstallForcelist, found=" + policyValue));
					return;
				}
			} else if (policyName === "KeyPermissions") {
				let value = JSON.parse(policyValue);
				if (!value.hasOwnProperty(extensionID)) {
					reject(new Error("Unexpected policy value for KeyPermissions, found=" + value));
					return;
				}
				allowCorporateKeyUsage = value[extensionID].allowCorporateKeyUsage;
			}
		}
		resolve(allowCorporateKeyUsage);
	});`, &allowCorporateKeyUsage); err != nil {
		errors.Wrap(err, "failed to execute Javascript in chrome://policy/")
	}
	if allowCorporateKeyUsage != expectAccess {
		errors.Errorf("KeyPermissions has different value expected: %t, found: %t", expectAccess, allowCorporateKeyUsage)
	}
	return nil
}

func forceUpdateChromeExtensions(ctx context.Context, br *browser.Browser, conn *chrome.Conn) error {
	const extensionID = "hoppbgdeajkagempifacalpdapphfoai" // PlatformKeys Test Extension

	if err := conn.Navigate(ctx, "chrome://extensions"); err != nil {
		return errors.Wrap(err, "failed to connect to chrome://extensions/ page")
	}

	// Connect to Test API to use it with the UI library.
	tconn, err := br.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	ui := uiauto.New(tconn)

	// Developer mode toggle button might be on/off, try two times.
	const maxCalls = 2
	numCalls := 0
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		numCalls++
		if numCalls > maxCalls {
			return testing.PollBreak(errors.New("break the poll, max calls reached"))
		}
		if err := uiauto.Combine("Update chrome extensions",
			ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Developer mode")),
			ui.WithTimeout(time.Minute).WaitUntilExists(nodewith.Role(role.Button).Name("Update")),
			ui.LeftClick(nodewith.Role(role.Button).Name("Update")),
			ui.WaitUntilExists(nodewith.Role(role.StaticText).Name("ID: "+extensionID)),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to update chrome extensions")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to update chrome extensions")
	}

	return nil
}
