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
	"chromiumos/tast/local/chrome"
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
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:enrollment"},
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
		chrome.KeepState(),
		chrome.ExtraArgs("--disable-policy-key-verification"))
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
		name     string                 // name is the subtest name.
		policy   *policy.SystemTimezone // policy is the policy we test.
		timezone string                 // timezone is a short string of the timezone set by the policy.
	}{
		{
			name:     "Berlin",
			policy:   &policy.SystemTimezone{Val: "Europe/Berlin"},
			timezone: "Europe/Berlin",
		},
		{
			name:     "Tokyo",
			policy:   &policy.SystemTimezone{Val: "Asia/Tokyo"},
			timezone: "Asia/Tokyo",
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
				chrome.KeepState(),
				chrome.ExtraArgs("--disable-policy-key-verification"))
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}

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
		})
	}
}
