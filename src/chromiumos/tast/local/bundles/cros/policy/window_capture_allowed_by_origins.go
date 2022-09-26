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
	// windowCaptureAllowedByOriginsHTML is the file containing the HTML+JS code exercising getDisplayMedia().
	windowCaptureAllowedByOriginsHTML = "window_capture_allowed_by_origins.html"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowCaptureAllowedByOrigins,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of WindowCaptureAllowedByOrigins policy",
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
		Data: []string{windowCaptureAllowedByOriginsHTML},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ScreenCaptureAllowed{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.WindowCaptureAllowedByOrigins{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// WindowCaptureAllowedByOrigins tests the namesake policy.
func WindowCaptureAllowedByOrigins(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, tc := range []struct {
		name               string
		wantCaptureBlocked bool
		wantScreenTab      bool
		displaySurface     string          // Passed to the getDisplayMedia() call. See https://www.w3.org/TR/screen-capture/#displaycapturesurfacetype.
		policies           []policy.Policy // list of policies to be set.
	}{
		{
			name:               "no_policies_allow_all",
			wantCaptureBlocked: false,
			wantScreenTab:      true,
			displaySurface:     "window",
		},
		{
			name:               "not_set_block",
			wantCaptureBlocked: true,
			displaySurface:     "window",
			policies: []policy.Policy{
				&policy.ScreenCaptureAllowed{Val: false},
			},
		},
		{
			name:               "set_allow_window",
			wantCaptureBlocked: false,
			wantScreenTab:      false,
			displaySurface:     "window",
			policies: []policy.Policy{
				&policy.ScreenCaptureAllowed{Val: true},
				&policy.WindowCaptureAllowedByOrigins{Val: []string{server.URL}},
			},
		},
		{
			name:               "set_allow_monitor",
			wantCaptureBlocked: false,
			wantScreenTab:      false,
			displaySurface:     "monitor",
			policies: []policy.Policy{
				&policy.ScreenCaptureAllowed{Val: true},
				&policy.WindowCaptureAllowedByOrigins{Val: []string{server.URL}},
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
			conn, err := br.NewConn(ctx, server.URL+"/"+windowCaptureAllowedByOriginsHTML)
			if err != nil {
				s.Fatal("Failed to connect to the window capture page: ", err)
			}
			defer conn.Close()

			if err := conn.Call(ctx, nil, "start", tc.displaySurface); err != nil {
				s.Fatal("Failed to call start(): ", err)
			}

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+tc.name)

			mediaPicker := nodewith.Role(role.Window).ClassName("DesktopMediaPickerDialogView")
			screenTab := nodewith.Name("Entire Screen").ClassName("Tab").Ancestor(mediaPicker)
			windowTab := nodewith.Name("Window").ClassName("Tab").Ancestor(mediaPicker)
			tabTab := nodewith.NameRegex(regexp.MustCompile("(Chrome|Chromium) Tab")).ClassName("Tab").Ancestor(mediaPicker)
			timeout := 5 * time.Second

			if tc.wantCaptureBlocked {
				// No media picker should come up.
				if err := ui.EnsureGoneFor(mediaPicker, timeout)(ctx); err != nil {
					s.Fatal("A media picker dialog appeared even though screen capture was disallowed: ", err)
				}
			} else {
				if err := ui.WithTimeout(timeout).WaitUntilExists(mediaPicker)(ctx); err != nil {
					s.Fatal("Failed to find a media picker: ", err)
				}

				if err := ui.WithTimeout(timeout).WaitUntilExists(tabTab)(ctx); err != nil {
					s.Fatal("Failed to find a tab for selecting chrome tabs: ", err)
				}

				if err := ui.WithTimeout(timeout).WaitUntilExists(windowTab)(ctx); err != nil {
					s.Fatal("Failed to find a tab for selecting windows: ", err)
				}

				if tc.wantScreenTab {
					if err := ui.WithTimeout(timeout).WaitUntilExists(screenTab)(ctx); err != nil {
						s.Fatal("Failed to find a tab for selecting entire screen: ", err)
					}
				} else {
					if err := ui.EnsureGoneFor(screenTab, timeout)(ctx); err != nil {
						s.Fatal("Media picker should not have a tab for selecting the entire screen: ", err)
					}
				}
			}
		})
	}

}
