// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TranslateEnabled,
		Desc: "Behavior of Translate policy, checking if the translate widget shows up or not dependent on the policy setting",
		Contacts: []string{
			"marcgrimme@google.com", // Test author
			"kathrelkeld@chromium.org",
			"chromeos-commercial-managed-user-experience@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"translate_enabled_page_fr.html"},
	})
}

// TranslateEnabled validates the UI behaviour of the different
// states the policy introduces. When enabled/unset the translate widget
// appears otherwise it should not appear. The correct UI behaviours are
// checked.
func TranslateEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

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
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Setup Chrome and enable policy
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Provide more data in artefacts if test fails.
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			// Open the browser and navigate to the to be translated page.
			url := server.URL + "/translate_enabled_page_fr.html"
			conn, err := cr.NewConn(ctx, url)
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
				s.Fatal("Failed to wait for location change: ", err)
			}

			// Find the translate node and validate against error.
			foundTranslate, err := ui.Exists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Translate this page",
			})
			if err != nil {
				s.Fatal("Error during checking for UI Compontent to translate: ", err)
			}

			if foundTranslate != param.wantTranslate {
				s.Errorf("Wrong visibility for translated gadget: got %t; want %t", foundTranslate, param.wantTranslate)
			}
		})
	}
}
