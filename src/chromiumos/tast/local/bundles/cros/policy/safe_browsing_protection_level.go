// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SafeBrowsingProtectionLevel,
		Desc: "Checks if Google Chrome's Safe Browsing feature is enabled and the mode it operates in",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.User,
	})
}

func SafeBrowsingProtectionLevel(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	securityPageElement := `document.querySelector("body > settings-ui").shadowRoot.querySelector("#main").shadowRoot.querySelector("settings-basic-page").shadowRoot.querySelector("#basicPage > settings-section.expanded > settings-privacy-page").shadowRoot.querySelector("#security > settings-security-page")`
	standardElement := securityPageElement + `.shadowRoot.querySelector("#safeBrowsingStandard")`
	radioGroupElement := securityPageElement + `.shadowRoot.querySelector("#safeBrowsingRadioGroup")`

	for _, param := range []struct {
		name           string
		wantRestricted bool   // wantRestricted is the wanted restriction state of the checkboxes in Safe Browsing settings page.
		selectedOption string // selectedOption is the selected safety level in Safe Browsing settings page.
		value          *policy.SafeBrowsingProtectionLevel
	}{
		{
			name:           "unset",
			wantRestricted: false,
			selectedOption: "1",
			value:          &policy.SafeBrowsingProtectionLevel{Stat: policy.StatusUnset},
		},
		{
			name:           "no_protection",
			wantRestricted: true,
			selectedOption: "2",
			value:          &policy.SafeBrowsingProtectionLevel{Val: 0},
		},
		{
			name:           "standard_protection",
			wantRestricted: true,
			selectedOption: "1",
			value:          &policy.SafeBrowsingProtectionLevel{Val: 1},
		},
		{
			name:           "enhanced_protection",
			wantRestricted: true,
			selectedOption: "0",
			value:          &policy.SafeBrowsingProtectionLevel{Val: 2},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open settings page where the affected checkboxes can be found.
			conn, err := cr.NewConn(ctx, "chrome://settings/security")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// Wait for the page to be loaded.
			if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
				s.Fatal("Waiting load failed: ", err)
			}

			// Check if the radio buttons are disabled.
			var isRestricted bool
			if err := conn.Eval(ctx, standardElement+`.disabled`, &isRestricted); err != nil {
				s.Fatal("Failed to evaluate the JS expression checking the restriction behavior: ", err)
			}
			if isRestricted != param.wantRestricted {
				s.Fatalf("Failed to verify restriction behavior; got %s, want %s", isRestricted, param.wantRestricted)
			}

			// Check the selected safety level.
			var selctedOption string
			if err := conn.Eval(ctx, radioGroupElement+`.selected`, &selctedOption); err != nil {
				s.Fatal("Failed to evaluate the JS expression checking the selected browsing safety level option hehavior: ", err)
			}
			if selctedOption != param.selectedOption {
				s.Fatalf("Failed to verify the selected browsing safety level option behavior; got %s, want %s", selctedOption, param.selectedOption)
			}
		})
	}
}
