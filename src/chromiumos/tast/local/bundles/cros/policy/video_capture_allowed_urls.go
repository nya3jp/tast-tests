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

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoCaptureAllowedUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of VideoCaptureAllowedUrls policy, checking that allow URLs don't request for video capture access",
		Contacts: []string{
			"eariassoto@google.com", // Test author
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
		Data: []string{"video_capture_allowed.html"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.VideoCaptureAllowedUrls{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// VideoCaptureAllowedUrls tests the VideoCaptureAllowedUrls policy.
func VideoCaptureAllowedUrls(ctx context.Context, s *testing.State) {
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
		policy      *policy.VideoCaptureAllowedUrls
	}{
		{
			name:        "include_url",
			expectedAsk: false,
			policy:      &policy.VideoCaptureAllowedUrls{Val: []string{server.URL + "/video_capture_allowed.html"}},
		},
		{
			name:        "exclude_url",
			expectedAsk: true,
			policy:      &policy.VideoCaptureAllowedUrls{Val: []string{"https://my_corp_site.com/conference.html"}},
		},
		{
			name:        "unset",
			expectedAsk: true,
			policy:      &policy.VideoCaptureAllowedUrls{Stat: policy.StatusUnset},
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
			conn, err := br.NewConn(ctx, server.URL+"/video_capture_allowed.html")
			if err != nil {
				s.Error("Failed to open website: ", err)
			}
			defer conn.Close()

			allowButton := nodewith.Name("Allow").Role(role.Button)
			if param.expectedAsk {
				if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(allowButton)(ctx); err != nil {
					s.Error("Failed to find the video capture prompt dialog: ", err)
				}
			} else {
				if err := ui.EnsureGoneFor(allowButton, 10*time.Second)(ctx); err != nil {
					s.Error("Failed to make sure no video capture prompt dialog shows: ", err)
				}
			}
		})
	}
}
