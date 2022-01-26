// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	//"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	//"chromiumos/tast/local/chrome/browser/browserfixt"
	//"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	// htmlFile is the file containing the HTML+JS code exercising getDisplayMedia().
	htmlFile = "getdisplaymedia.html"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowCaptureAllowedByOrigins,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Behavior of WindowCaptureAllowedByOrigins policy",
		Contacts: []string{
			"dandrader@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyWindowCaptureLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedInWindowCapture,
			Val:               browser.TypeLacros,
		}},
		Data: []string{
			htmlFile,
			"third_party/blackframe.js",
		},
	})
}

// WindowCaptureAllowedByOrigins tests the namesake policy.
func WindowCaptureAllowedByOrigins(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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
			wantCaptureBlocked: true,
			policies: []policy.Policy{
				&policy.ScreenCaptureAllowed{Val: false},
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

			// Open the test page.
			conn, err := cr.NewConn(ctx, server.URL+"/"+htmlFile)
			if err != nil {
				s.Fatal("Failed to connect to the window capture page: ", err)
			}
			defer conn.Close()
			//defer conn.CloseTarget(ctx)

			if err := conn.Call(ctx, nil, "start", "window"); err != nil && !tc.wantCaptureBlocked {
				s.Fatal("Window capture failed: ", err)
			}
		})
	}

}
