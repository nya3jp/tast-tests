// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchWithDeviceEphemeralUsersEnabled,
		Desc: "Checks that Kiosk configuration starts correctly with DeviceEphemeralUsersEnabled policy set to true",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alt-modalities-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func LaunchWithDeviceEphemeralUsersEnabled(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		// https://crbug.com/1202902 combining DeviceEphemeralUsersEnabled
		// with Kiosk autolaunch caused Kiosk not starting successfully.
		kioskmode.AddExtraPolicies([]policy.Policy{&policy.DeviceEphemeralUsersEnabled{Val: true}}),
		kioskmode.AutoLaunch(kioskmode.KioskAppAccountID),
	)
	if err != nil {
		s.Error("Failed to start Chrome in Kiosk mode: ", err)
	}

	// Serving and refreshing of empty policies list is necessary because of
	// the AutoLaunch option used for Kiosk mode. If policies are only cleaned
	// before starting new Chrome instance then Kiosk mode starts
	// automatically.
	defer policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{})
	defer cr.Close(ctx)
}
