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
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer func(ctx context.Context) {
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

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
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicy(&policy.DeviceGuestModeEnabled{Val: false})
			pb.AddLegacyDevicePolicy("device_off_hours.intervals", param.intervals)
			pb.AddLegacyDevicePolicy("device_off_hours.timezone", "Europe/Berlin")
			pb.AddLegacyDevicePolicy("device_off_hours.ignored_policy_proto_tags", []int{guestModeEnabledIdx})

			// Update policies.
			if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Read updated policies.
			dutPolicies, err := policyutil.PoliciesFromDUT(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get device policies: ", err)
			}

			// Check availability of DeviceGuestModeEnabled policy.
			guestModeShouldBeSet := !param.active
			_, ok := dutPolicies.Chrome[guestModeName]
			if ok != guestModeShouldBeSet {
				s.Errorf("Invalid DeviceGuestModeEnabled policy availability: expected %t; got %t", guestModeShouldBeSet, ok)
			}
		})
	}
}
