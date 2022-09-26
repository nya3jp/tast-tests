// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
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
		Func:         WebUSBBlockedForUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of WebUsbBlockedForUrls policy, checking that blocked URLs don't request for access to a USB device",
		Contacts: []string{
			"adikov@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}, {
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}},
		Data: []string{"web_usb_blocked.html"},
	})
}

// WebUSBBlockedForUrls tests the WebUsbBlockedForUrls policy.
func WebUSBBlockedForUrls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Error("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	for _, param := range []struct {
		name        string
		expectedAsk bool // expectedAsk states whether a dialog to ask for permission should appear or not.
		policy      *policy.WebUsbBlockedForUrls
	}{
		{
			name:        "include_url",
			expectedAsk: false,
			policy:      &policy.WebUsbBlockedForUrls{Val: []string{server.URL + "/web_usb_blocked.html"}},
		},
		{
			name:        "exclude_url",
			expectedAsk: true,
			policy:      &policy.WebUsbBlockedForUrls{Val: []string{"https://my_corp_site.com/conference.html"}},
		},
		{
			name:        "unset",
			expectedAsk: true,
			policy:      &policy.WebUsbBlockedForUrls{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Error("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Error("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Error("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Open the test website.
			conn, err := br.NewConn(ctx, server.URL+"/web_usb_blocked.html")
			if err != nil {
				s.Error("Failed to open website: ", err)
			}
			defer conn.Close()

			if err := ui.LeftClick(nodewith.ClassName("btn"))(ctx); err != nil {
				s.Fatal("Failed to right click the button: ", err)
			}

			cancelButton := nodewith.Name("Cancel").Role(role.Button)
			if param.expectedAsk {
				if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(cancelButton)(ctx); err != nil {
					s.Error("Failed to find the USB prompt dialog: ", err)
				}
			} else {
				if err := ui.EnsureGoneFor(cancelButton, 10*time.Second)(ctx); err != nil {
					s.Error("Failed to make sure no USB prompt dialog shows: ", err)
				}
			}
		})
	}
}
