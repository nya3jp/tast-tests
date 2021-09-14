// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package phonehub

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/crossdevice/phonehub"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SilencePhone,
		Desc: "Checks that toggling Phone Hub's \"Silence phone\" pod will toggle do-not-disturb on the Android phone",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:cross-device"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboarded",
	})
}

// SilencePhone tests toggling the connected Android phone's do-not-disturb setting using Phone Hub.
func SilencePhone(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice
	if err := phonehub.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Phone Hub: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Time to wait for Do Not Disturb status changes to take effect across devices.
	dndTimeout := 5 * time.Second

	// Make sure Do Not Disturb is initially turned off, and confirm the state of the Silence Phone pod in the Phone Hub UI.
	if err := androidDevice.ToggleDoNotDisturb(ctx, false); err != nil {
		s.Fatal("Failed to ensure Do Not Disturb is initially turned off: ", err)
	}
	if err := phonehub.WaitForPhoneSilenced(ctx, tconn, false /*enabled*/, dndTimeout); err != nil {
		s.Fatal("Failed waiting for Silence Phone pod to be disabled in Phone Hub: ", err)
	}
	// Also make sure it's turned off after the test.
	defer androidDevice.ToggleDoNotDisturb(ctx, false)

	// Toggle Do Not Disturb on and off from Phone Hub, while checking the actual setting value on the Android.
	if err := phonehub.ToggleSilencePhonePod(ctx, tconn, true); err != nil {
		s.Fatal("Failed to turn on Do Not Disturb from Phone Hub: ", err)
	}
	if err := androidDevice.WaitForDoNotDisturb(ctx, true /*enabled*/, dndTimeout); err != nil {
		s.Fatal("Failed waiting for Do Not Disturb to be enabled on the Android device: ", err)
	}
	if err := phonehub.ToggleSilencePhonePod(ctx, tconn, false); err != nil {
		s.Fatal("Failed to turn off Do Not Disturb from Phone Hub: ", err)
	}
	if err := androidDevice.WaitForDoNotDisturb(ctx, false /*enabled*/, dndTimeout); err != nil {
		s.Fatal("Failed waiting for Do Not Disturb to be disabled on the Android device: ", err)
	}

	// Toggle it on/off from the Android phone, and make sure Phone Hub's display is updated.
	if err := androidDevice.ToggleDoNotDisturb(ctx, true); err != nil {
		s.Fatal("Failed to turn on Do Not Disturb from Android: ", err)
	}
	if err := phonehub.WaitForPhoneSilenced(ctx, tconn, true /*enabled*/, dndTimeout); err != nil {
		s.Fatal("Failed waiting for Silence Phone pod to be enabled in Phone Hub: ", err)
	}
	if err := androidDevice.ToggleDoNotDisturb(ctx, false); err != nil {
		s.Fatal("Failed to turn off Do Not Disturb from Android: ", err)
	}
	if err := phonehub.WaitForPhoneSilenced(ctx, tconn, false /*enabled*/, dndTimeout); err != nil {
		s.Fatal("Failed waiting for Silence Phone pod to be disabled in Phone Hub: ", err)
	}
}
