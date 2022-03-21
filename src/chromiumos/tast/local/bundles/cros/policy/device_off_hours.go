// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	empb "chromiumos/policy/chromium/policy/enterprise_management_proto"
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

	const guestModeEnabledIdx = 3 // see components/policy/proto/chrome_device_policy.proto
	const guestModeName = "DeviceGuestModeEnabled"

	// alwaysOff: Intervals covering the whole week.
	alwaysOff := []*empb.WeeklyTimeIntervalProto{
		{
			Start: &empb.WeeklyTimeProto{DayOfWeek: &[]empb.WeeklyTimeProto_DayOfWeek{1}[0], Time: &[]int32{0}[0]},
			End:   &empb.WeeklyTimeProto{DayOfWeek: &[]empb.WeeklyTimeProto_DayOfWeek{7}[0], Time: &[]int32{0}[0]},
		},
		{
			Start: &empb.WeeklyTimeProto{DayOfWeek: &[]empb.WeeklyTimeProto_DayOfWeek{7}[0], Time: &[]int32{0}[0]},
			End:   &empb.WeeklyTimeProto{DayOfWeek: &[]empb.WeeklyTimeProto_DayOfWeek{1}[0], Time: &[]int32{0}[0]},
		},
	}
	// neverOff: Interval covering no time at all.
	neverOff := []*empb.WeeklyTimeIntervalProto{
		{
			Start: &empb.WeeklyTimeProto{DayOfWeek: &[]empb.WeeklyTimeProto_DayOfWeek{1}[0], Time: &[]int32{0}[0]},
			End:   &empb.WeeklyTimeProto{DayOfWeek: &[]empb.WeeklyTimeProto_DayOfWeek{1}[0], Time: &[]int32{0}[0]},
		},
	}

	for _, param := range []struct {
		name      string                          // subtest name.
		intervals []*empb.WeeklyTimeIntervalProto // off hours intervals.
		active    bool                            // Whether or not we expect off-hours to be active
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

			pb.DeviceProto = &empb.ChromeDeviceSettingsProto{}
			proto := empb.DeviceOffHoursProto{}
			proto.Intervals = param.intervals
			proto.Timezone = &[]string{"Europe/Berlin"}[0]
			proto.IgnoredPolicyProtoTags = []int32{guestModeEnabledIdx}
			pb.DeviceProto.DeviceOffHours = &proto

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
