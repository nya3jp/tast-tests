// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
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

const (
	// htmlFile is the file containing the HTML+JS code exercising getDisplayMedia().
	tapCaptureAllowedByOriginsHTML = "tap_capture_allowed_by_origins.html"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabCaptureAllowedByOrigins,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of TabCaptureAllowedByOrigins policy",
		Contacts: []string{
			"dandrader@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{tapCaptureAllowedByOriginsHTML},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.TabCaptureAllowedByOrigins{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.ScreenCaptureAllowed{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// TabCaptureAllowedByOrigins tests the namesake policy.
func TabCaptureAllowedByOrigins(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, tc := range []struct {
		name               string
		wantCaptureBlocked bool
		policies           []policy.Policy // list of policies to be set.
	}{
		{
			name:               "not_set_block",
			wantCaptureBlocked: true,
			policies: []policy.Policy{
				&policy.ScreenCaptureAllowed{Val: false},
			},
		},
		{
			name:               "set_allow",
			wantCaptureBlocked: false,
			policies: []policy.Policy{
				&policy.ScreenCaptureAllowed{Val: false},
				&policy.TabCaptureAllowedByOrigins{Val: []string{server.URL}},
			},
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// Open the test page.
			conn, err := br.NewConn(ctx, server.URL+"/"+tapCaptureAllowedByOriginsHTML)
			if err != nil {
				s.Fatal("Failed to connect to the window capture page: ", err)
			}
			defer conn.Close()

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+tc.name)

			ui := uiauto.New(tconn)

			if tc.wantCaptureBlocked {
				// No media picker should come up
				if err := ui.EnsureGoneFor(nodewith.Role(role.Window).ClassName("DesktopMediaPickerDialogView"), 10*time.Second)(ctx); err != nil {
					s.Fatal("A media picker dialog appeared even though screen capture was disallowed: ", err)
				}
			} else {
				// We expect the media picker dialog to allow *only* tabs to be selected (and not also windows and the entire desktop).
				// A tabs-only media picker has a particular title, whereas the general one is "Choose what to share".
				tabOnlyPicker := nodewith.NameRegex(regexp.MustCompile("Share a (Chromium|Chrome) tab")).Role(role.Window).ClassName("DesktopMediaPickerDialogView")
				if err := ui.WaitUntilExists(tabOnlyPicker)(ctx); err != nil {
					s.Fatal("Failed to find a tabs-only media picker: ", err)
				}
			}
		})
	}

}
