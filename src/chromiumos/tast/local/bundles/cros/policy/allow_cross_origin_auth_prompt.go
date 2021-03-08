// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AllowCrossOriginAuthPrompt,
		Desc: "Checks the behavior of 3rd party images on pages whether it shows auth prompt or not",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"allow_cross_origin_auth_prompt_index.html"},
	})
}

func AllowCrossOriginAuthPrompt(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, param := range []struct {
		name       string
		wantPrompt bool                               // wantPrompt is to check if the page should show an authentication prompt.
		value      *policy.AllowCrossOriginAuthPrompt // value is the value of the policy.
	}{
		{
			name:       "enabled",
			wantPrompt: true,
			value:      &policy.AllowCrossOriginAuthPrompt{Val: true},
		},
		{
			name:       "disabled",
			wantPrompt: false,
			value:      &policy.AllowCrossOriginAuthPrompt{Val: false},
		},
		{
			name:       "unset",
			wantPrompt: false,
			value:      &policy.AllowCrossOriginAuthPrompt{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}

			// Open a page that contains a 3rd party image which requires authentication.
			// TODO(mohamedaomar) crbug/1178509: host internal image with basic auth instead of the external link used in the html page.
			if err := conn.Navigate(ctx, server.URL+"/allow_cross_origin_auth_prompt_index.html"); err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Create a uiauto.Context with default timeout.
			ui := uiauto.New(tconn)

			promptWindow := nodewith.Name("Sign in").Role(role.Window)

			if param.wantPrompt {
				if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(promptWindow)(ctx); err != nil {
					s.Fatal("Failed to find the prompt window for authentication: ", err)
				}
			} else {
				if err := ui.EnsureGoneFor(promptWindow, 10*time.Second)(ctx); err != nil {
					s.Fatal("Failed to make sure no authentication prompt window popup: ", err)
				}
			}
		})
	}
}
