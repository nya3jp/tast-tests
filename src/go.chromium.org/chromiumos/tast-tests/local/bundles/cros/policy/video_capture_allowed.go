// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"go.chromium.org/chromiumos/tast-tests/common/fixture"
	"go.chromium.org/chromiumos/tast-tests/common/policy"
	"go.chromium.org/chromiumos/tast-tests/common/policy/fakedms"
	"go.chromium.org/chromiumos/tast/ctxutil"
	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/browser"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/browser/browserfixt"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/role"
	"go.chromium.org/chromiumos/tast-tests/local/policyutil"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoCaptureAllowed,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of VideoCaptureAllowed policy, checking if a website is allowed to capture video",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
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
	})
}

// VideoCaptureAllowed tests the VideoCaptureAllowed policy.
func VideoCaptureAllowed(ctx context.Context, s *testing.State) {
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
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)

	for _, param := range []struct {
		name          string
		expectedBlock bool // expectedBlock states whether a dialog to ask for permission should appear or not.
		policy        *policy.VideoCaptureAllowed
	}{
		{
			name:          "unset",
			expectedBlock: false,
			policy:        &policy.VideoCaptureAllowed{Stat: policy.StatusUnset},
		},
		{
			name:          "blocked",
			expectedBlock: true,
			policy:        &policy.VideoCaptureAllowed{Val: false},
		},
		{
			name:          "allowed",
			expectedBlock: false,
			policy:        &policy.VideoCaptureAllowed{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Open the test website.
			conn, err := br.NewConn(ctx, server.URL+"/video_capture_allowed.html")
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			// Check for existence of either the allow or block button until one of them appears.
			allowButton := nodewith.Name("Allow").Role(role.Button)
			blockedButton := nodewith.Name("This page has been blocked from accessing your camera.").Role(role.Button)
			blocked := false
			if err := testing.Poll(ctx, func(ctx context.Context) error {

				if err = ui.Exists(allowButton)(ctx); err == nil {
					return testing.PollBreak(nil)
				}

				if err = ui.Exists(blockedButton)(ctx); err == nil {
					blocked = true
					return testing.PollBreak(nil)
				}

				return errors.New("failed to find allow or blocked button")

			}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
				s.Fatal("Failed to find indicator if video capture is allowed or blocked: ", err)
			}

			if blocked != param.expectedBlock {
				s.Errorf("Unexpected blocking of video capture: want %t got %t", param.expectedBlock, blocked)
			}
		})
	}
}
