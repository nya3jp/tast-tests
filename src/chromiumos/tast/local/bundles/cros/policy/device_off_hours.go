// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceOffHours,
		Desc: "Behavior of DeviceOffHours policy",
		Contacts: []string{
			"rbock@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

// See src/platform/tast-tests/src/chromiumos/tast/common/policy/defs.go
const guestModeEnabledIdx = 124

// alwaysOff: Intervals covering the whole week
var alwaysOff = []*policy.RefWeeklyTimeIntervals{
	{
		Start: &policy.RefWeeklyTime{DayOfWeek: "Monday"},
		End:   &policy.RefWeeklyTime{DayOfWeek: "Sunday"},
	},
	{
		Start: &policy.RefWeeklyTime{DayOfWeek: "Sunday"},
		End:   &policy.RefWeeklyTime{DayOfWeek: "Monday"},
	},
}

// neverOff: Interval covering no time at all
var neverOff = []*policy.RefWeeklyTimeIntervals{
	{
		Start: &policy.RefWeeklyTime{DayOfWeek: "Monday"},
		End:   &policy.RefWeeklyTime{DayOfWeek: "Monday"},
	},
}

func DeviceOffHours(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		//chrome.NoLogin(), // FIXME: NoLogin instead of FakeLogin times out
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment()) // FIXME: Why do we need KeepEnrollment?
	if err != nil {
		s.Fatal("Starting chrome failed: ", err)
	}

	// FIXME: Would be lovely to have a wrapper for this
	defer func(ctx context.Context) {
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	for _, param := range []struct {
		name      string                           // subtest name.
		intervals []*policy.RefWeeklyTimeIntervals // off hours intervals.
		active    bool                             // Whether or not we expect off-hours to be active
	}{
		{
			name:      "ActiveOffHours",
			intervals: alwaysOff,
			active:    true,
		},
		{
			name:      "InactiveOffHours",
			intervals: neverOff,
			active:    false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			offHours := &policy.DeviceOffHours{
				Val: &policy.DeviceOffHoursValue{
					IgnoredPolicyProtoTags: []int{guestModeEnabledIdx},
					Intervals:              param.intervals,
					Timezone:               "Europe/Berlin",
				},
			}

			policies := []policy.Policy{
				offHours,
				&policy.DeviceGuestModeEnabled{Val: false},
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// FIXME: Write bug: Why does the generic OffHours policy log-out of guest sessions?
			// That should be the job of the session manager or whoever.

			// If off-hours are active, guest mode for the verification below
			if param.active {
				policies[1] = &policy.DeviceGuestModeEnabled{Stat: policy.StatusUnset}
			}

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			if err := policyutil.Verify(ctx, tconn, policies); err != nil {
				s.Fatal("Failed to verify policies: ", err)
			}
		})
	}
}
