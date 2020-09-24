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
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TranslateEnabled,
		Desc: "Behavior of Translate policy",
		Contacts: []string{
			"marcgrimme@google.com", // Test author
			"kathrelkeld@chromium.org",
			"chromeos-commercial-managed-user-experience@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"translate_page_fr.html"},
	})
}

// TranslateEnabled validates the UI behaviour of the different
// states the policy introduces. When enabled/unset the translate widget
// appears otherwise it should not appear. The correct UI behaviours are
// checked.
func TranslateEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.TranslateEnabled
	}{
		{
			name:  "true",
			value: &policy.TranslateEnabled{Val: true},
		},
		{
			name:  "false",
			value: &policy.TranslateEnabled{Val: false},
		},
		{
			name:  "unset",
			value: &policy.TranslateEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Setup and start webserver (implicitly provides data form above)
			server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
			defer server.Close()

			// Setup Chrome and enable policy
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Connect to Test API to use it with the ui library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			// provide more data in artefacts if test fails.
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			// open the browser and navigate to the to be translated page
			url := server.URL + "/translate_page_fr.html"
			conn, err := cr.NewConn(ctx, url)
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
				s.Fatal("Failed to wait for location change. Error: ", err)
			}

			// find the translate node and validate against error.
			isTranslate, _ := ui.Exists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Translate this page",
			})

			wantTranslate := (param.value.Stat == policy.StatusUnset) || param.value.Val

			if isTranslate != wantTranslate {
				s.Errorf("Wrong visibility for translated gadget: got %t; want %t", isTranslate, wantTranslate)
			}
		})
	}
}
