// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package smartlock contains tests for the Smart Lock feature in ChromeOS.
package smartlock

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Unlock,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Signs into ChromeOS, locks device and then unlocks it with Smart Lock",
		Contacts: []string{
			"dhaddock@chromium.org",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		Attr:         []string{"group:cross-device"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboarded",
	})
}

// Unlock tests unlocking ChromeOS using Smart Lock feature.
func Unlock(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice

	// Smart Lock requires the Android phone to have a PIN code. Set it here and defer removing it.
	if err := androidDevice.SetPIN(ctx); err != nil {
		s.Fatal("Failed to set lock screen PIN on Android: ", err)
	}
	defer androidDevice.ClearPIN(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Locking the ChromeOS screen")
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen on ChromeOS: ", err)
	}

	s.Log("Waiting for locked screen state to be verified")
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	s.Log("Waiting for the Smart Lock ready indicator")
	if err := lockscreen.WaitForSmartUnlockReady(ctx, tconn); err != nil {
		s.Fatal("Failed waiting for Smart Lock icon to turn green: ", err)
	}

	s.Log("Smart Unlock available. Clicking user image to log back in")
	if err := lockscreen.ClickUserImage(ctx, tconn); err != nil {
		s.Fatal("Failed to click user image on the ChromeOS lock screen: ", err)
	}

	// Check for shelf to ensure we are logged back in.
	if err := ash.WaitForShelf(ctx, tconn, 10*time.Second); err != nil {
		s.Fatal("Shelf did not appear after logging in: ", err)
	}
}
