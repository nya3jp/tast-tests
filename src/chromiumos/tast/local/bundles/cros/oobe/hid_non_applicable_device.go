// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"chromiumos/tast/local/oobe"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidNonApplicableDevice,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that OOBE HID Detection screen is skipped on non-applicable devices",
		Contacts: []string{
			"andrewdear@google.com",
			"cros-connectivity@google.com",
		},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SkipOnFormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
		Fixture:      "chromeEnterOobeHidDetection",
		Timeout:      time.Second * 15,
	})
}

// HidNonApplicableDevice checks that the OOBE Welcome screen is shown.
func HidNonApplicableDevice(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*oobe.ChromeOobeHidDetection).Chrome
	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	if err := oobe.IsWelcomeScreenVisible(ctx, oobeConn); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}
}
