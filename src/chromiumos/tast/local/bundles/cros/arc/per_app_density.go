// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/perappdensity"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerAppDensity,
		Desc:         "Checks that density can be changed with Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

const perAppDensityApk = "ArcPerAppDensityTest.apk"

func PerAppDensity(ctx context.Context, s *testing.State) {
	const (
		packageName = "org.chromium.arc.testapp.perappdensitytest"
		// The following scale factors have been taken from TaskRecordArc.
		increasedSF = 1.1
		decreasedSF = 0.9
		// Defined in XML files in vendor/google_arc/packages/developments/ArcPerAppDensityTest/res/layout.
		squareSidePx = 100
	)

	displayDensity, err := perappdensity.InitDisplay(ctx, s)
	if err != nil {
		s.Fatal("Error initializing display: ", err)
	}
	expectedInitialPixelCount := (displayDensity * squareSidePx) * (displayDensity * squareSidePx)

	// Ensure that density is restored to initial state.
	defer func() {
		initialState := perappdensity.DensityChange{"reset", "ctrl+0", expectedInitialPixelCount}

		ew, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Error creating keyboard: ", err)
		}
		defer ew.Close()

		if err := initialState.ExecuteChange(ctx, cr, ew); err != nil {
			s.Fatal("Error with performing: ", err)
		}
	}()

	testSteps := []perappdensity.DensityChange{
		{
			"increase",
			"ctrl+=",
			expectedInitialPixelCount * float64(increasedSF) * float64(increasedSF),
		},
		{
			"reset",
			"ctrl+0",
			expectedInitialPixelCount,
		},
		{
			"decrease",
			"ctrl+-",
			expectedInitialPixelCount * float64(decreasedSF) * float64(decreasedSF),
		}}

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := arc.BootstrapCommand(ctx, perappdensity.Setprop, perappdensity.DensitySetting, "true").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}
	defer arc.BootstrapCommand(ctx, perappdensity.Setprop, perappdensity.DensitySetting, "false").Run(testexec.DumpLogOnError)

	testing.ContextLog(ctx, "Installing app")
	if err := a.Install(ctx, arc.APKPath(perAppDensityApk)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to set tablet mode to false: ", err)
	}
	defer cleanup(ctx)

	if err := perappdensity.RunTest(ctx, s, packageName, testSteps, []string{".ViewActivity", ".SurfaceViewActivity"}, expectedInitialPixelCount); err != nil {
		s.Fatal("Failed running density test: ", err)
	}

}
