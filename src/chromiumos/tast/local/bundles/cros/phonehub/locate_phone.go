// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package phonehub

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/crossdevice/phonehub"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocatePhone,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that toggling Phone Hub's \"Locate phone\" pod will toggle the ringer on the Android phone",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		Attr:         []string{"group:cross-device"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboardedAllFeatures",
		Timeout:      2 * time.Minute,
	})
}

// LocatePhone tests toggling the connected Android phone's "Locate phone" feature using Phone Hub.
func LocatePhone(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := phonehub.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Phone Hub: ", err)
	}

	// Time to wait for "Locate phone" status changes to take effect across devices.
	locateTimeout := 5 * time.Second

	// Disable the pod if it's already active.
	active, err := phonehub.LocatePhoneEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check initial 'Locate phone' status: ", err)
	}
	if active {
		if err := phonehub.ToggleLocatePhonePod(ctx, tconn, false /*enable*/); err != nil {
			s.Fatal("Failed to turn off 'Locate phone': ", err)
		}

		if err := androidDevice.WaitForFindMyPhone(ctx, false /*active*/, locateTimeout); err != nil {
			s.Fatal("Failed waiting for 'Find my phone' alarm to be inactive on the phone: ", err)
		}
	}

	if err := phonehub.ToggleLocatePhonePod(ctx, tconn, true /*enable*/); err != nil {
		s.Fatal("Failed to turn on 'Locate phone': ", err)
	}
	// Defer toggling the Android screen off and back on to silence the ringer in case of failure.
	defer androidDevice.ToggleScreen(cleanupCtx)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := androidDevice.WaitForFindMyPhone(ctx, true /*active*/, locateTimeout); err != nil {
		s.Fatal("Failed waiting for 'Find my phone' alarm to be active on the phone: ", err)
	}

	if err := phonehub.ToggleLocatePhonePod(ctx, tconn, false /*enable*/); err != nil {
		s.Fatal("Failed to turn off 'Locate phone': ", err)
	}

	if err := androidDevice.WaitForFindMyPhone(ctx, false /*active*/, locateTimeout); err != nil {
		s.Fatal("Failed waiting for 'Find my phone' alarm to be inactive on the phone: ", err)
	}
}
