// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multidevice

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/multidevice"
	"chromiumos/tast/local/chrome/multidevice/phonehub"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PhoneHubDoNotDisturb,
		Desc: "Checks toggling the connected Android's do-not-disturb setting using Phone Hub",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "multidevice",
		Timeout:      5 * time.Minute,
	})
}

// PhoneHubDoNotDisturb tests toggling the connected Android phone's do-not-disturb setting using Phone Hub.
func PhoneHubDoNotDisturb(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*multidevice.FixtData).TestConn
	connectedDevice := s.FixtValue().(*multidevice.FixtData).ConnectedDevice
	if err := phonehub.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Phone Hub: ", err)
	}

	// Make sure Do Not Disturb is initially turned off, and confirm the state of the Silence Phone pod in the Phone Hub UI.
	if err := connectedDevice.ToggleDoNotDisturb(ctx, false); err != nil {
		s.Fatal("Failed to ensure Do Not Disturb is initially turned off: ", err)
	}
	if silenced, err := phonehub.PhoneSilenced(ctx, tconn); err != nil {
		s.Fatal("Failed to get Silence Phone status from Phone Hub: ", err)
	} else if silenced {
		s.Fatal("Silence Phone UI pod unexpectedly enabled after turning off Do Not Disturb on the phone")
	}
	// Also make sure it's turned off after the test.
	defer connectedDevice.ToggleDoNotDisturb(ctx, false)

	// Toggle Do Not Disturb on and off from Phone Hub, while checking the actual setting value on the Android.
	dndTimeout := 5 * time.Second
	if err := phonehub.ToggleSilencePhonePod(ctx, tconn, true); err != nil {
		s.Fatal("Failed to turn on Do Not Disturb from Phone Hub: ", err)
	}
	if err := connectedDevice.WaitForDoNotDisturb(ctx, true /*enabled*/, dndTimeout); err != nil {
		s.Fatal("Failed waiting for Do Not Disturb to be enabled on the Android device")
	}
	if err := phonehub.ToggleSilencePhonePod(ctx, tconn, false); err != nil {
		s.Fatal("Failed to turn off Do Not Disturb from Phone Hub: ", err)
	}
	if err := connectedDevice.WaitForDoNotDisturb(ctx, false /*enabled*/, dndTimeout); err != nil {
		s.Fatal("Failed waiting for Do Not Disturb to be disabled on the Android device")
	}

	// Toggle it on/off from the Android phone, and make sure Phone Hub's display is updated.
	if err := connectedDevice.ToggleDoNotDisturb(ctx, true); err != nil {
		s.Fatal("Failed to turn on Do Not Disturb from Android: ", err)
	}
	if err := phonehub.WaitForPhoneSilenced(ctx, tconn, true /*enabled*/, dndTimeout); err != nil {
		s.Fatal("Failed waiting for Silence Phone pod to be enabled in Phone Hub")
	}
	if err := connectedDevice.ToggleDoNotDisturb(ctx, false); err != nil {
		s.Fatal("Failed to turn off Do Not Disturb from Android: ", err)
	}
	if err := phonehub.WaitForPhoneSilenced(ctx, tconn, false /*enabled*/, dndTimeout); err != nil {
		s.Fatal("Failed waiting for Silence Phone pod to be disabled in Phone Hub")
	}
}
