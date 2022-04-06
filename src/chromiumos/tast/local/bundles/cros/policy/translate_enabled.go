// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TranslateEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of Translate policy, checking if the translate widget shows up or not dependent on the policy setting",
		Contacts: []string{
			"marcgrimme@google.com", // Test author
			"kathrelkeld@chromium.org",
			"chromeos-commercial-managed-user-experience@google.com",
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
		Data: []string{"translate_enabled_page_fr.html"},
	})
}

// TranslateEnabled validates the UI behaviour of the different
// states the policy introduces. When enabled/unset the translate widget
// appears otherwise it should not appear. The correct UI behaviours are
// checked.
func TranslateEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Setup and start webserver (implicitly provides data form above)
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, param := range []struct {
		// name is the subtest name.
		name          string
		wantTranslate bool
		// value is the policy value.
		value *policy.TranslateEnabled
	}{
		{
			name:          "true",
			wantTranslate: true,
			value:         &policy.TranslateEnabled{Val: true},
		},
		{
			name:          "false",
			wantTranslate: false,
			value:         &policy.TranslateEnabled{Val: false},
		},
		{
			name:          "unset",
			wantTranslate: true,
			value:         &policy.TranslateEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// Provide more data in artefacts if test fails.
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			// Open the browser and navigate to the to be translated page.
			url := server.URL + "/translate_enabled_page_fr.html"
			conn, err := br.NewConn(ctx, url)
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			ui := uiauto.New(tconn)
			if err := ui.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
				s.Fatal("Failed to wait for location change: ", err)
			}

			// Find the translate node and validate against error.
			foundTranslate, err := ui.IsNodeFound(ctx, nodewith.Role(role.Button).Name("Translate this page"))
			if err != nil {
				s.Fatal("Error during checking for UI Compontent to translate: ", err)
			}

			if foundTranslate != param.wantTranslate {
				s.Errorf("Wrong visibility for translated gadget: got %t; want %t", foundTranslate, param.wantTranslate)
			}
		})
	}
}
