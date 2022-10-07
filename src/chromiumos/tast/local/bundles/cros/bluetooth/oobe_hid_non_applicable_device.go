// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeHidNonApplicableDevice,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that OOBE HID Detection screen is skipped on non Chromebox, Chromebase, or Chromebit form factors",
		Contacts: []string{
			"andrewdear@google.com",
			"cros-connectivity@google.com",
		},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeEnterOobeHidDetection",
		Timeout:      time.Second * 15,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnFormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
			},
		},
	})
}

func OobeHidNonApplicableDevice(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(bluetooth.FixtData).Chrome

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	// Check that the Welcome screen is visible.
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}

}
