// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceOffHours,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of DeviceOffHours policy",
		Contacts: []string{
			"rbock@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:commercial_limited"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DeviceGuestModeEnabled{}, pci.VerifiedFunctionalityOS),
			pci.SearchFlagWithName("DeviceOffHours", pci.VerifiedFunctionalityOS),
		},
	})
}

func DeviceOffHours(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
	if err != nil {
		s.Fatal("Starting chrome failed: ", err)
	}
	defer func(ctx context.Context) {
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	type RefWeeklyTime struct {
		DayOfWeek int `json:"day_of_week"`
		Time      int `json:"time"`
	}
	type RefWeeklyTimeIntervals struct {
		End   *RefWeeklyTime `json:"end"`
		Start *RefWeeklyTime `json:"start"`
	}

	const guestModeEnabledIdx = 3 // see components/policy/proto/chrome_device_policy.proto
	const guestModeName = "DeviceGuestModeEnabled"

	// alwaysOff: Intervals covering the whole week.
	alwaysOff := []*RefWeeklyTimeIntervals{
		{
			Start: &RefWeeklyTime{DayOfWeek: 1, Time: 0},
			End:   &RefWeeklyTime{DayOfWeek: 7, Time: 0},
		},
		{
			Start: &RefWeeklyTime{DayOfWeek: 7, Time: 0},
			End:   &RefWeeklyTime{DayOfWeek: 1, Time: 0},
		},
	}
	// neverOff: Interval covering no time at all.
	neverOff := []*RefWeeklyTimeIntervals{
		{
			Start: &RefWeeklyTime{DayOfWeek: 1, Time: 0},
			End:   &RefWeeklyTime{DayOfWeek: 1, Time: 0},
		},
	}

	for _, param := range []struct {
		name      string                    // subtest name.
		intervals []*RefWeeklyTimeIntervals // off hours intervals.
		active    bool                      // Whether or not we expect off-hours to be active
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
			pb := policy.NewBlob()
			pb.AddPolicy(&policy.DeviceGuestModeEnabled{Val: false})
			pb.AddLegacyDevicePolicy("device_off_hours.intervals", param.intervals)
			pb.AddLegacyDevicePolicy("device_off_hours.timezone", "Europe/Berlin")
			pb.AddLegacyDevicePolicy("device_off_hours.ignored_policy_proto_tags", []int{guestModeEnabledIdx})

			// Update policies.
			if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// The OffHoursPolicy itself triggers another change of policies if active.
			// On slow devices, this update might not be immediately available after
			// the refresh. Thus, let's try a few times.
			guestModeShouldBeSet := !param.active
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Read updated policies.
				dutPolicies, err := policyutil.PoliciesFromDUT(ctx, tconn)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get device policies"))
				}

				// Check availability of DeviceGuestModeEnabled policy.
				guestModeIsSet := false
				_, guestModeIsSet = dutPolicies.Chrome[guestModeName]
				if guestModeIsSet != guestModeShouldBeSet {
					return errors.Errorf("invalid DeviceGuestModeEnabled policy availability: expected %t; got %t", guestModeShouldBeSet, guestModeIsSet)
				}
				return nil
			}, &testing.PollOptions{
				Timeout:  30 * time.Second,
				Interval: 100 * time.Millisecond,
			}); err != nil {
				s.Error("Policies do not have the expected values: ", err)
			}
		})
	}
}
