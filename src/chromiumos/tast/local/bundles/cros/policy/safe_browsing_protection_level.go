// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
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

	settingsSecurityPageElement := `document.querySelector("body > settings-ui").shadowRoot.querySelector("#main").shadowRoot.querySelector("settings-basic-page").shadowRoot.querySelector("#basicPage > settings-section.expanded > settings-privacy-page").shadowRoot.querySelector("#security > settings-security-page")`
	safeBrowsingStandardElement := settingsSecurityPageElement + `.shadowRoot.querySelector("#safeBrowsingStandard")`
	safeBrowsingRadioGroupElement := settingsSecurityPageElement + `.shadowRoot.querySelector("#safeBrowsingRadioGroup")`

	for _, param := range []struct {
		name           string
		wantRestricted bool
		wantSelected   string
		value          *policy.SafeBrowsingProtectionLevel
	}{
		{
			name:           "unset",
			wantRestricted: false,
			wantSelected:   "1",
			value:          &policy.SafeBrowsingProtectionLevel{Stat: policy.StatusUnset},
		},
		{
			name:           "No Protection",
			wantRestricted: true,
			wantSelected:   "2",
			value:          &policy.SafeBrowsingProtectionLevel{Val: 0},
		},
		{
			name:           "Standard Protection",
			wantRestricted: true,
			wantSelected:   "1",
			value:          &policy.SafeBrowsingProtectionLevel{Val: 1},
		},
		{
			name:           "Enhanced Protection",
			wantRestricted: true,
			wantSelected:   "0",
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

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Check if the radio buttons are disabled.
				var isRestricted bool
				if err := conn.Eval(ctx, safeBrowsingStandardElement+`.disabled`, &isRestricted); err != nil {
					return errors.Wrap(err, "failed to evaluate the JS expression checking the restriction behavior")
				}
				if isRestricted != param.wantRestricted {
					return errors.Errorf("failed to verify restriction behavior: got %s; want %s", isRestricted, param.wantRestricted)
				}
				return nil
			}, &testing.PollOptions{
				Timeout: 15 * time.Second,
			}); err != nil {
				s.Fatal("Did not find the radio button safeBrowsingStandard: ", err)
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Check the selected safety level.
				var selctedOption string
				if err := conn.Eval(ctx, safeBrowsingRadioGroupElement+`.selected`, &selctedOption); err != nil {
					return errors.Wrap(err, "failed to evaluate the JS expression checking the selected browsing safety level option hehavior")
				}
				if selctedOption != param.wantSelected {
					return errors.Errorf("failed to verify the selected browsing safety level option behavior: got %s; want %s", selctedOption, param.wantSelected)
				}
				return nil
			}, &testing.PollOptions{
				Timeout: 15 * time.Second,
			}); err != nil {
				s.Fatal("Did not find the safeBrowsingRadioGroup element: ", err)
			}
		})
	}
}
