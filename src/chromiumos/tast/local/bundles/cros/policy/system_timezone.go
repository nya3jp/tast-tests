// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemTimezone,
		Desc: "Behavior of SystemTimezone policy",
		Contacts: []string{
			"vsavu@google.com",          // Test author
			"alexanderhartl@google.com", // Original author of the remote test.
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func SystemTimezone(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	defer func(ctx context.Context) {
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	for _, param := range []struct {
		name            string                 // name is the subtest name.
		policy          *policy.SystemTimezone // policy is the policy we test.
		timezone        string                 // timezone is a short string of the timezone set by the policy.
		wantRestriction ui.RestrictionState
		selectedOption  string
	}{
		{
			name:            "berlin",
			policy:          &policy.SystemTimezone{Val: "Europe/Berlin"},
			timezone:        "Europe/Berlin",
			wantRestriction: ui.RestrictionDisabled,
			selectedOption:  "Choose from list",
		},
		{
			name:            "tokyo",
			policy:          &policy.SystemTimezone{Val: "Asia/Tokyo"},
			timezone:        "Asia/Tokyo",
			wantRestriction: ui.RestrictionDisabled,
			selectedOption:  "Choose from list",
		},
		{
			name:            "unset",
			policy:          &policy.SystemTimezone{Stat: policy.StatusUnset},
			wantRestriction: ui.RestrictionNone,
			selectedOption:  "Set automatically",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Close the previous Chrome instance.
			if err := cr.Close(ctx); err != nil {
				s.Error("Failed to close Chrome connection: ", err)
			}

			// Restart Chrome.
			cr, err = chrome.New(ctx,
				chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
				chrome.DMSPolicy(fdms.URL),
				chrome.KeepEnrollment())
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}

			if param.timezone != "" {
				// Wait until the timezone is set.
				if err := testing.Poll(ctx, func(ctx context.Context) error {

					out, err := os.Readlink("/var/lib/timezone/localtime")
					if err != nil {
						return errors.Wrap(err, "failed to get the timezone")
					}

					if !strings.Contains(string(out), param.timezone) {
						return errors.Errorf("unexpected timezone: got %q; want %q", string(out), param.timezone)
					}

					return nil
				}, &testing.PollOptions{
					Timeout: 30 * time.Second,
				}); err != nil {
					s.Error("Failed to get the expected timezone: ", err)
				}
			}

			// Open the timezone settings page and check restrictions.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/dateTime/timeZone")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// Connect to Test API to use it with the UI library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			// Find the radio group node.
			rgNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeRadioGroup}, 15*time.Second)
			if err != nil {
				s.Fatal("Finding radio group failed: ", err)
			}
			defer rgNode.Release(ctx)

			// Find the selected radio button under the radio group.
			srbNode, err := rgNode.FindSelectedRadioButton(ctx)
			if err != nil {
				s.Fatal("Finding the selected radio button failed: ", err)
			}
			defer srbNode.Release(ctx)

			if err := policyutil.CheckNodeAttributes(srbNode, ui.FindParams{
				Attributes: map[string]interface{}{
					"restriction": param.wantRestriction,
					"name":        param.selectedOption,
				},
			}); err != nil {
				s.Error("Unexpected settings state: ", err)
			}
		})
	}
}
