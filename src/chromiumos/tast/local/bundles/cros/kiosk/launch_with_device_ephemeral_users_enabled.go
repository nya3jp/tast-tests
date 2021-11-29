// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchWithDeviceEphemeralUsersEnabled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Kiosk configuration starts correctly with DeviceEphemeralUsersEnabled policy set to true",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.KioskAutoLaunchCleanup,
		Params: []testing.Param{
			{
				Name: "ash",
				Val:  chrome.ExtraArgs(""),
			},
			{
				Name: "lacros",
				Val:  chrome.ExtraArgs("--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore"),
			},
		},
	})
}

func LaunchWithDeviceEphemeralUsersEnabled(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	chromeOptions := s.Param().(chrome.Option)
	kiosk, _, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		// https://crbug.com/1202902 combining DeviceEphemeralUsersEnabled
		// with Kiosk autolaunch caused Kiosk not starting successfully.
		kioskmode.ExtraPolicies([]policy.Policy{&policy.DeviceEphemeralUsersEnabled{Val: true}}),
		kioskmode.ExtraChromeOptions(chromeOptions),
		kioskmode.AutoLaunch(kioskmode.KioskAppAccountID),
	)
	if err != nil {
		s.Error("Failed to start Chrome in Kiosk mode: ", err)
	}

	defer kiosk.Close(ctx)
}
