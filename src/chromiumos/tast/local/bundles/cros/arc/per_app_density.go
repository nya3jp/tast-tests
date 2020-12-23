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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PerAppDensity,
		Desc:     "Checks that density can be changed with Android applications",
		Contacts: []string{"sarakato@chromium.org", "arc-framework+tast@google.com"},
		// TODO(b/150909711): Enable this test after fix.
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBooted",
	})
}

func PerAppDensity(ctx context.Context, s *testing.State) {
	const (
		packageName = "org.chromium.arc.testapp.perappdensitytest"
		// The following scale factors have been taken from TaskRecordArc.
		increasedSF = 1.1
		decreasedSF = 0.9
		// Defined in XML files in vendor/google_arc/packages/developments/ArcPerAppDensityTest/res/layout.
		squareSidePx = 100
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	dd, err := perappdensity.MeasureDisplayDensity(ctx, a)
	if err != nil {
		s.Fatal("Error obtaining initial display density: ", err)
	}

	if err := perappdensity.SetUpApk(ctx, a, perappdensity.Apk); err != nil {
		s.Fatal("Failed to setup perAppDensityApk: ", err)
	}
	defer arc.BootstrapCommand(ctx, perappdensity.Setprop, perappdensity.Setting, "false").Run(testexec.DumpLogOnError)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to set tablet mode to false: ", err)
	}
	defer cleanup(ctx)

	expectedInitialPixelCount := (dd * squareSidePx) * (dd * squareSidePx)
	testSteps := []perappdensity.Change{
		{
			Name:            "increase",
			KeySequence:     "ctrl+=",
			BlackPixelCount: expectedInitialPixelCount * float64(increasedSF) * float64(increasedSF),
		},
		{
			Name:            "reset",
			KeySequence:     "ctrl+0",
			BlackPixelCount: expectedInitialPixelCount,
		},
		{
			Name:            "decrease",
			KeySequence:     "ctrl+-",
			BlackPixelCount: expectedInitialPixelCount * float64(decreasedSF) * float64(decreasedSF),
		}}

	for _, activity := range []string{".ViewActivity", ".SurfaceViewActivity"} {
		// Start each activity, and execute the density changes for each activity.
		testing.ContextLogf(ctx, "Running %q", activity)

		if err := perappdensity.RunTest(ctx, cr, a, packageName, testSteps, activity, expectedInitialPixelCount); err != nil {
			s.Fatalf("On %q, failed to run density test: %v", activity, err)
		}
	}

}
