// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemTimezone,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of SystemTimezone policy",
		Contacts: []string{
			"vsavu@google.com",          // Test author
			"alexanderhartl@google.com", // Original author of the remote test.
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.SystemTimezone{}, pci.VerifiedFunctionalityUI),
		},
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
		name            string                  // name is the subtest name.
		policy          *policy.SystemTimezone  // policy is the policy we test.
		timezone        string                  // timezone is a short string of the timezone set by the policy.
		wantRestriction restriction.Restriction // wantRestriction is the wanted restriction state of the radio buttons.
		selectedOption  string
	}{
		{
			name:            "berlin",
			policy:          &policy.SystemTimezone{Val: "Europe/Berlin"},
			timezone:        "Europe/Berlin",
			wantRestriction: restriction.Disabled,
			selectedOption:  "Choose from list",
		},
		{
			name:            "tokyo",
			policy:          &policy.SystemTimezone{Val: "Asia/Tokyo"},
			timezone:        "Asia/Tokyo",
			wantRestriction: restriction.Disabled,
			selectedOption:  "Choose from list",
		},
		{
			name:            "unset",
			policy:          &policy.SystemTimezone{Stat: policy.StatusUnset},
			wantRestriction: restriction.None,
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

			// Open the time zone settings page.
			if err := policyutil.OSSettingsPage(ctx, cr, "dateTime/timeZone").
				SelectNode(ctx, nodewith.
					Role(role.RadioButton).
					Name(param.selectedOption)).
				Checked(checked.True).
				Restriction(param.wantRestriction).
				Verify(); err != nil {
				s.Error("Unexpected OS settings state: ", err)
			}
		})
	}
}
